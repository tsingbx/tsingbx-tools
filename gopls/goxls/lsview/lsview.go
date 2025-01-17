// Copyright 2022 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package lsview

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"sort"
	"time"

	"golang.org/x/tools/internal/fakenet"
	"golang.org/x/tools/internal/jsonrpc2"
)

func Main(app, goxls string, args ...string) {
	flag := flag.NewFlagSet("lsview", flag.ExitOnError)
	var (
		fSummary = flag.Bool("s", false, "show summary information for jsonrpc")
		fSel     = flag.String("sel", "", "select a method to print")
	)
	flag.Parse(args)
	sel := *fSel

	fin, err := os.Open("gopls.in")
	check(err)
	defer fin.Close()

	fview, err := os.Create("gopls.view")
	check(err)
	defer fview.Close()

	fdiff, err := os.Create("gopls.diff")
	check(err)
	defer fdiff.Close()

	logd := log.New(fdiff, "", log.LstdFlags)
	log := log.New(io.MultiWriter(os.Stderr, fview), "", log.LstdFlags)
	reqStream := jsonrpc2.NewHeaderStream(fakenet.NewConn("request", fin, os.Stdout))
	reqChan := make(chan jsonrpc2.ID, 1)
	respChan := make(chan *jsonrpc2.Response, 1)
	reqChan2 := make(chan jsonrpc2.ID, 1)
	respChan2 := make(chan *jsonrpc2.Response, 1)

	go func() {
		ctx := context.Background()
		for {
			msg, _, err := reqStream.Read(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					time.Sleep(time.Second / 5)
					continue
				}
				check(err)
			}
			switch req := msg.(type) {
			case *jsonrpc2.Call:
				id, method := req.ID(), req.Method()
				summary := *fSummary && method != sel
				log.Printf("[%v] %s\n%s", id, method, params(req.Params(), summary))
				reqChan <- id
				resp, ret := respFetch(respChan, summary)
				if resp != nil {
					log.Printf("[%v] %s ret\n%s", id, app, resp)
				}
				if goxls != "" {
					var resp2, ret2 any
					select { // allow send request failed
					case <-time.After(time.Second):
					case reqChan2 <- id:
						if resp2, ret2 = respFetch(respChan2, summary); resp2 != nil {
							log.Printf("[%v] %s ret\n%s", id, goxls, resp2)
						}
					}
					if eq, resp, resp2 := checkEqual(id, ret, ret2, resp, resp2); !eq {
						logd.Printf("[%v] %s\n%s", id, method, params(req.Params(), summary))
						logd.Printf("[%v] %s ret\n%s", id, app, resp)
						logd.Printf("[%v] %s ret\n%s", id, goxls, resp2)
					}
				}
			case *jsonrpc2.Notification:
				method := req.Method()
				summary := *fSummary && method != sel
				log.Printf("[] %s:\n%s", method, params(req.Params(), summary))
			}
		}
	}()
	go respLoop(app, respChan, reqChan)
	if goxls != "" {
		go respLoop(goxls, respChan2, reqChan2)
	}
	select {}
}

func respFetch(respChan chan *jsonrpc2.Response, summary bool) (any, any) {
	select {
	case <-time.After(time.Second):
	case resp := <-respChan:
		ret := any(resp.Err())
		if ret == nil {
			return paramsEx(resp.Result(), summary)
		}
		return fmt.Sprintf("%serror: %v\n", indent, ret), ret
	}
	return nil, nil
}

func respLoop(app string, respChan chan *jsonrpc2.Response, reqChan chan jsonrpc2.ID) {
	if app == "" {
		return
	}
	fout, err := os.Open(app + ".out")
	check(err)
	defer fout.Close()

	ctx := context.Background()
	respStream := jsonrpc2.NewHeaderStream(fakenet.NewConn("response", fout, os.Stdout))
	resps := make([]*jsonrpc2.Response, 0, 8)
next:
	id := <-reqChan
	for i, resp := range resps {
		if resp.ID() == id {
			resps = append(resps[i:], resps[i+1:]...)
			respChan <- resp
			goto next
		}
	}
	for {
		msg, _, err := respStream.Read(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				time.Sleep(time.Second / 5)
				continue
			}
			check(err)
		}
		switch resp := msg.(type) {
		case *jsonrpc2.Response:
			if resp.ID() == id {
				respChan <- resp
				goto next
			}
			resps = append(resps, resp)
		}
	}
}

type any = interface{}
type mapt = map[string]any
type slice = []any

const indent = "  "

func params(raw json.RawMessage, summary bool) []byte {
	if summary {
		return nil
	}
	var ret any
	err := json.Unmarshal(raw, &ret)
	if err != nil {
		return raw
	}
	return paramsFmt(ret, indent, false)
}

func paramsEx(raw json.RawMessage, summary bool) ([]byte, any) {
	var ret any
	err := json.Unmarshal(raw, &ret)
	if err != nil {
		return raw, ret
	}
	return paramsFmt(ret, indent, summary), ret
}

func paramsFmt(ret any, prefix string, summary bool) []byte {
	if summary {
		return nil
	}
	var b bytes.Buffer
	switch val := ret.(type) {
	case mapt:
		keys := keys(val)
		for _, k := range keys {
			v := val[k]
			if isComplex(v) {
				fmt.Fprintf(&b, "%s%s:\n%s", prefix, k, paramsFmt(v, prefix+indent, false))
			} else {
				fmt.Fprintf(&b, "%s%s: %v\n", prefix, k, v)
			}
		}
	case slice:
		if isComplexSlice(val) {
			for _, v := range val {
				s, _ := json.Marshal(v)
				fmt.Fprintf(&b, "%s%s\n", prefix, s)
			}
		} else {
			fmt.Fprintf(&b, "%s%v\n", prefix, val)
		}
	default:
		log.Panicln("unexpected:", reflect.TypeOf(ret))
	}
	return b.Bytes()
}

func isComplexSlice(v slice) bool {
	if len(v) > 0 {
		return isComplex(v[0])
	}
	return false
}

func isComplex(v any) bool {
	if _, ok := v.(mapt); ok {
		return true
	}
	_, ok := v.(slice)
	return ok
}

func checkEqual(id jsonrpc2.ID, a, b, resp, resp2 any) (eq bool, fmta, fmtb any) {
	ma, oka := a.(mapt)
	mb, okb := b.(mapt)
	if oka && okb {
		if id == jsonrpc2.NewIntID(0) {
			delete(ma, "serverInfo")
			delete(mb, "serverInfo")
		}
		if eq = mapEqual(ma, mb); !eq {
			fmta, fmtb = paramsFmt(a, indent, false), paramsFmt(b, indent, false)
		}
		return
	}
	return reflect.DeepEqual(a, b), resp, resp2
}

func mapEqual(ma, mb mapt) bool {
	for k, va := range ma {
		if vb, ok := mb[k]; ok {
			mva, oka := va.(mapt)
			mvb, okb := vb.(mapt)
			eq := false
			if oka && okb {
				eq = mapEqual(mva, mvb)
			} else {
				eq = reflect.DeepEqual(va, vb)
			}
			if eq {
				delete(ma, k)
				delete(mb, k)
			}
		}
	}
	return len(ma) == 0 && len(mb) == 0
}

func keys(v mapt) []string {
	ret := make([]string, 0, len(v))
	for key := range v {
		ret = append(ret, key)
	}
	sort.Strings(ret)
	return ret
}

func check(err error) {
	if err != nil {
		log.Panicln(err)
	}
}
