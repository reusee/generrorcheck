package main

/*
package main

func foo() error {
	return nil
}

func main() {
	_ = foo()

	func() error {
		return nil
	}()

	foo()

	func() (int, error, string) {
		return 0, nil, "foo"
	}()
}
*/

/*
package main

func foo() error {
	return nil
}

func main() {
	_ = foo()
	err = func() error {
		return nil
	}()
	if err != nil {
		panic(err)
	}
	err = foo()
	if err != nil {
		panic(err)
	}
	_, err, _ = func() (int, error, string) {
		return 0, nil, "foo"
	}()
	if err != nil {
		panic(err)
	}
}
*/

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"log"
	"os"

	"golang.org/x/tools/go/types"
)

var (
	ft = log.Fatal
	pt = fmt.Printf
)

type Visitor func(node ast.Node) Visitor

func (v Visitor) Visit(node ast.Node) ast.Visitor {
	if v != nil {
		return v(node)
	}
	return nil
}

func main() {
	fileSet := token.NewFileSet()
	f, err := parser.ParseFile(fileSet, os.Args[1], nil, parser.ParseComments)
	if err != nil {
		ft(err)
	}
	var config types.Config
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	_, err = config.Check("main", fileSet, []*ast.File{f}, info)
	if err != nil {
		ft(err)
	}

	errorType := types.New("error")
	var visitor Visitor
	visitor = func(node ast.Node) Visitor {
		block, ok := node.(*ast.BlockStmt)
		if !ok {
			return visitor
		}
		return Visitor(func(node ast.Node) Visitor {
			stmt, ok := node.(*ast.ExprStmt)
			if !ok {
				return visitor
			}
			return Visitor(func(node ast.Node) Visitor {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return visitor
				}
				what := info.Types[call.Fun].Type
				returns := what.(*types.Signature).Results()
				var returnIdents []ast.Expr
				hasError := false
				for i := 0; i < returns.Len(); i++ {
					if returns.At(i).Type() != errorType {
						returnIdents = append(returnIdents, ast.NewIdent("_"))
					} else {
						hasError = true
						returnIdents = append(returnIdents, ast.NewIdent("err"))
					}
				}
				if hasError {
					for i, st := range block.List {
						if st != stmt {
							continue
						}
						block.List[i] = &ast.AssignStmt{
							Lhs: returnIdents,
							Tok: token.ASSIGN,
							Rhs: []ast.Expr{call},
						}
						var newList []ast.Stmt
						newList = append(newList, block.List[:i+1]...)
						newList = append(newList, &ast.IfStmt{
							Cond: &ast.BinaryExpr{
								X:  ast.NewIdent("err"),
								Op: token.NEQ,
								Y:  ast.NewIdent("nil"),
							},
							Body: &ast.BlockStmt{
								List: []ast.Stmt{
									&ast.ExprStmt{
										X: &ast.CallExpr{
											Fun:  ast.NewIdent("panic"),
											Args: []ast.Expr{ast.NewIdent("err")},
										},
									},
								},
							},
						})
						newList = append(newList, block.List[i+1:]...)
						block.List = newList
						break
					}
				}
				return nil
			})
		})
	}
	ast.Walk(visitor, f)

	var buf bytes.Buffer
	if err := format.Node(&buf, fileSet, f); err != nil {
		ft(err)
	}
	pt("%s\n", buf.Bytes())
}
