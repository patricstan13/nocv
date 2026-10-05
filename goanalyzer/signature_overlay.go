package goanalyzer

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

func buildSignatureOverlay(callable resolvedCallable, proposed resolvedSignature) (string, []byte, error) {
	functionType, declarationFile, err := findCallableDeclaration(callable)
	if err != nil {
		return "", nil, err
	}

	declarationFile, err = filepath.Abs(declarationFile)
	if err != nil {
		return "", nil, fmt.Errorf("resolve declaration file for %s: %w", callable.node.Ref, err)
	}
	sourceContents, err := os.ReadFile(declarationFile)
	if err != nil {
		return "", nil, fmt.Errorf("read declaration file for %s: %w", callable.node.Ref, err)
	}

	replacementStart, replacementEnd, err := signatureSourceRange(callable, functionType, len(sourceContents))
	if err != nil {
		return "", nil, err
	}
	replacement := renderSourceSignature(callable.signature, proposed)
	overlaySource := make([]byte, 0, len(sourceContents)-(replacementEnd-replacementStart)+len(replacement))
	overlaySource = append(overlaySource, sourceContents[:replacementStart]...)
	overlaySource = append(overlaySource, replacement...)
	overlaySource = append(overlaySource, sourceContents[replacementEnd:]...)
	return declarationFile, overlaySource, nil
}

// findCallableDeclaration resolves package functions and concrete methods from
// FuncDecls, and interface methods from named FuncType fields.
func findCallableDeclaration(callable resolvedCallable) (*ast.FuncType, string, error) {
	for _, source := range orderedFiles(callable.pkg) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && callable.pkg.TypesInfo.Defs[function.Name] == callable.function {
				return function.Type, source.name, nil
			}
		}

		var interfaceMethod *ast.FuncType
		ast.Inspect(source.file, func(node ast.Node) bool {
			field, ok := node.(*ast.Field)
			if !ok {
				return true
			}
			functionType, ok := field.Type.(*ast.FuncType)
			if !ok {
				return true
			}
			for _, name := range field.Names {
				if callable.pkg.TypesInfo.Defs[name] == callable.function {
					interfaceMethod = functionType
					return false
				}
			}
			return true
		})
		if interfaceMethod != nil {
			return interfaceMethod, source.name, nil
		}
	}
	return nil, "", fmt.Errorf("source declaration is unavailable for %s", callable.node.Ref)
}

func signatureSourceRange(callable resolvedCallable, functionType *ast.FuncType, sourceLength int) (int, int, error) {
	tokenFile := callable.pkg.Fset.File(functionType.Params.Opening)
	if tokenFile == nil {
		return 0, 0, fmt.Errorf("source positions are unavailable for %s", callable.node.Ref)
	}
	replacementStart := tokenFile.Offset(functionType.Params.Opening)
	replacementEndPosition := functionType.Params.End()
	if functionType.Results != nil {
		replacementEndPosition = functionType.Results.End()
	}
	replacementEnd := tokenFile.Offset(replacementEndPosition)
	if replacementStart < 0 || replacementEnd < replacementStart || replacementEnd > sourceLength {
		return 0, 0, fmt.Errorf("invalid declaration source range for %s", callable.node.Ref)
	}
	return replacementStart, replacementEnd, nil
}

func renderSourceSignature(current *types.Signature, proposed resolvedSignature) []byte {
	namedParameters := tupleHasNames(current.Params())
	parameters := make([]string, 0, len(proposed.parameterSources))
	for index, source := range proposed.parameterSources {
		if proposed.model.Variadic && index == len(proposed.parameterSources)-1 {
			source = "..." + source
		}
		name := ""
		if index < current.Params().Len() {
			name = current.Params().At(index).Name()
		} else if namedParameters {
			// Go tuples cannot mix named and unnamed entries.
			name = "_"
		}
		if name != "" {
			source = name + " " + source
		}
		parameters = append(parameters, source)
	}

	text := "(" + strings.Join(parameters, ", ") + ")"
	namedResults := tupleHasNames(current.Results())
	results := make([]string, 0, len(proposed.resultSources))
	for index, source := range proposed.resultSources {
		name := ""
		if index < current.Results().Len() {
			name = current.Results().At(index).Name()
		} else if namedResults {
			// Go tuples cannot mix named and unnamed entries.
			name = "_"
		}
		if name != "" {
			source = name + " " + source
		}
		results = append(results, source)
	}
	if len(results) == 1 && currentResultName(current, 0) == "" {
		text += " " + results[0]
	} else if len(results) != 0 {
		text += " (" + strings.Join(results, ", ") + ")"
	}
	return []byte(text)
}

func tupleHasNames(tuple *types.Tuple) bool {
	for index := 0; index < tuple.Len(); index++ {
		if tuple.At(index).Name() != "" {
			return true
		}
	}
	return false
}

func currentResultName(current *types.Signature, index int) string {
	if index >= current.Results().Len() {
		return ""
	}
	return current.Results().At(index).Name()
}
