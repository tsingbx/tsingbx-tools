// Copyright 2023 The GoPlus Authors (goplus.org). All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package analysis

import (
	"flag"
	"fmt"
	"reflect"

	"github.com/goplus/gop/ast"
	"github.com/goplus/gop/x/typesutil"
	"golang.org/x/tools/go/analysis"
)

// A GoAnalyzer describes a Go analyzer.
type GoAnalyzer = analysis.Analyzer

// A IAnalyzer abstracts a Go/Go+ analyzer.
type IAnalyzer interface {
	String() string
}

// Name returns analyzer name.
func Name(a IAnalyzer) string {
	return a.String()
}

// Doc returns analyzer Doc.
func Doc(a IAnalyzer) string {
	if i, ok := a.(*Analyzer); ok {
		return i.Doc
	}
	return a.(*GoAnalyzer).Doc
}

// URL returns analyzer URL.
func URL(a IAnalyzer) string {
	if i, ok := a.(*Analyzer); ok {
		return i.URL
	}
	return a.(*GoAnalyzer).URL
}

// Flags returns analyzer Flags.
func Flags(a IAnalyzer) *flag.FlagSet {
	if i, ok := a.(*Analyzer); ok {
		return &i.Flags
	}
	return &a.(*GoAnalyzer).Flags
}

// RunIsNil returns if analyzer's Run is nil or not.
func RunIsNil(a IAnalyzer) bool {
	if i, ok := a.(*Analyzer); ok {
		return i.Run == nil
	}
	return a.(*GoAnalyzer).Run == nil
}

// RunDespiteErrors returns analyzer RunDespiteErrors.
func RunDespiteErrors(a IAnalyzer) bool {
	if i, ok := a.(*Analyzer); ok {
		return i.RunDespiteErrors
	}
	return a.(*GoAnalyzer).RunDespiteErrors
}

// FactTypes returns analyzer FactTypes.
func FactTypes(a IAnalyzer) []Fact {
	if i, ok := a.(*Analyzer); ok {
		return i.FactTypes
	}
	return a.(*GoAnalyzer).FactTypes
}

// Requires returns analyzer Requires.
func Requires(a IAnalyzer) []IAnalyzer {
	if i, ok := a.(*Analyzer); ok {
		return i.Requires
	}
	reqs := a.(*GoAnalyzer).Requires
	ret := make([]IAnalyzer, len(reqs))
	for i, req := range reqs {
		ret[i] = req
	}
	return ret
}

// ForRequires visits Requires to call f(req) and stops if an error occurs.
func ForRequires(a IAnalyzer, f func(req IAnalyzer) error) error {
	if i, ok := a.(*Analyzer); ok {
		for _, req := range i.Requires {
			if e := f(req); e != nil {
				return e
			}
		}
	} else {
		for _, req := range a.(*GoAnalyzer).Requires {
			if e := f(req); e != nil {
				return e
			}
		}
	}
	return nil
}

// SetResult sets an analyzer result.
func SetResult(gopRet map[*Analyzer]any, goRet map[*GoAnalyzer]any, a IAnalyzer, v any) {
	if i, ok := a.(*Analyzer); ok {
		gopRet[i] = v
		return
	}
	goRet[a.(*GoAnalyzer)] = v
}

// An Analyzer describes an analysis function and its options.
type Analyzer struct {
	// The Name of the analyzer must be a valid Go identifier
	// as it may appear in command-line flags, URLs, and so on.
	Name string

	// Doc is the documentation for the analyzer.
	// The part before the first "\n\n" is the title
	// (no capital or period, max ~60 letters).
	Doc string

	// URL holds an optional link to a web page with additional
	// documentation for this analyzer.
	URL string

	// Flags defines any flags accepted by the analyzer.
	// The manner in which these flags are exposed to the user
	// depends on the driver which runs the analyzer.
	Flags flag.FlagSet

	// Run applies the analyzer to a package.
	// It returns an error if the analyzer failed.
	//
	// On success, the Run function may return a result
	// computed by the Analyzer; its type must match ResultType.
	// The driver makes this result available as an input to
	// another Analyzer that depends directly on this one (see
	// Requires) when it analyzes the same package.
	//
	// To pass analysis results between packages (and thus
	// potentially between address spaces), use Facts, which are
	// serializable.
	Run func(*Pass) (any, error)

	// RunDespiteErrors allows the driver to invoke
	// the Run method of this analyzer even on a
	// package that contains parse or type errors.
	// The Pass.TypeErrors field may consequently be non-empty.
	RunDespiteErrors bool

	// Requires is a set of Go/Go+ analyzers that must run successfully
	// before this one on a given package. This analyzer may inspect
	// the outputs produced by each analyzer in Requires.
	// The graph over analyzers implied by Requires edges must be acyclic.
	//
	// Requires establishes a "horizontal" dependency between
	// analysis passes (different analyzers, same package).
	Requires []IAnalyzer

	// ResultType is the type of the optional result of the Run function.
	ResultType reflect.Type

	// FactTypes indicates that this analyzer imports and exports
	// Facts of the specified concrete types.
	// An analyzer that uses facts may assume that its import
	// dependencies have been similarly analyzed before it runs.
	// Facts must be pointers.
	//
	// FactTypes establishes a "vertical" dependency between
	// analysis passes (same analyzer, different packages).
	FactTypes []Fact
}

func (a *Analyzer) String() string { return a.Name }

// A GoPass provides information to the Run function that
// applies a specific analyzer to a single Go package.
type GoPass = analysis.Pass

// A Pass provides information to the Run function that
// applies a specific analyzer to a single Go package.
//
// It forms the interface between the analysis logic and the driver
// program, and has both input and an output components.
//
// As in a compiler, one pass may depend on the result computed by another.
//
// The Run function should not call any of the Pass functions concurrently.
type Pass struct {
	GoPass

	// Analyzer is the identity of the current analyzer
	Analyzer *Analyzer

	// ResultOf provides the inputs to this analysis pass, which are
	// the corresponding results of its prerequisite analyzers.
	// The map keys are the elements of Analysis.Required,
	// and the type of each corresponding value is the required
	// analysis's ResultType.
	ResultOf map[*Analyzer]any

	// goxls: Go+
	GopFiles     []*ast.File     // the abstract syntax tree of each file
	GopTypesInfo *typesutil.Info // type information about the syntax trees
}

// PackageFact is a package together with an associated fact.
type PackageFact = analysis.PackageFact

// ObjectFact is an object together with an associated fact.
type ObjectFact = analysis.ObjectFact

// The Range interface provides a range. It's equivalent to and satisfied by
// ast.Node.
type Range = analysis.Range

func (pass *Pass) String() string {
	return fmt.Sprintf("%s@%s", pass.Analyzer.Name, pass.Pkg.Path())
}

// SetAnalyzer sets Analyzer of this pass.
func (pass *Pass) SetAnalyzer(a IAnalyzer) {
	if i, ok := a.(*Analyzer); ok {
		pass.Analyzer = i
		return
	}
	pass.GoPass.Analyzer = a.(*GoAnalyzer)
}

// A Fact is an intermediate fact produced during analysis.
//
// Each fact is associated with a named declaration (a types.Object) or
// with a package as a whole. A single object or package may have
// multiple associated facts, but only one of any particular fact type.
//
// A Fact represents a predicate such as "never returns", but does not
// represent the subject of the predicate such as "function F" or "package P".
//
// Facts may be produced in one analysis pass and consumed by another
// analysis pass even if these are in different address spaces.
// If package P imports Q, all facts about Q produced during
// analysis of that package will be available during later analysis of P.
// Facts are analogous to type export data in a build system:
// just as export data enables separate compilation of several passes,
// facts enable "separate analysis".
//
// Each pass (a, p) starts with the set of facts produced by the
// same analyzer a applied to the packages directly imported by p.
// The analysis may add facts to the set, and they may be exported in turn.
// An analysis's Run function may retrieve facts by calling
// Pass.Import{Object,Package}Fact and update them using
// Pass.Export{Object,Package}Fact.
//
// A fact is logically private to its Analysis. To pass values
// between different analyzers, use the results mechanism;
// see Analyzer.Requires, Analyzer.ResultType, and Pass.ResultOf.
//
// A Fact type must be a pointer.
// Facts are encoded and decoded using encoding/gob.
// A Fact may implement the GobEncoder/GobDecoder interfaces
// to customize its encoding. Fact encoding should not fail.
//
// A Fact should not be modified once exported.
type Fact = analysis.Fact