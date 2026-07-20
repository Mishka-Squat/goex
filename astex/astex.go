package astex

import (
	"go/ast"
	"iter"

	"deedles.dev/xiter"
	"github.com/Mishka-Squat/goex/gx"
	"github.com/Mishka-Squat/goex/slicesex"
	"github.com/Mishka-Squat/goex/stringsex"
)

func ExprTypeBaseFieldName(fieldType ast.Expr) (string, bool) {
	switch fieldType := fieldType.(type) {
	case *ast.Ident:
		return fieldType.Name, true
	case *ast.StarExpr:
		return ExprTypeBaseFieldName(fieldType.X)
	case *ast.SelectorExpr:
		if sel, ok := ExprGetFullTypeName(fieldType.Sel); ok {
			return sel, true
		}
	}

	return "", false
}

func ExprGetFullTypeName(fieldType ast.Expr) (string, bool) {
	switch fieldType := fieldType.(type) {
	case *ast.Ident:
		return fieldType.Name, true
	case *ast.StarExpr:
		return ExprGetFullTypeName(fieldType.X)
	case *ast.ArrayType:
		return ExprGetFullTypeName(fieldType.Elt)
	case *ast.SelectorExpr:
		if x, ok := ExprGetFullTypeName(fieldType.X); ok {
			if sel, ok := ExprGetFullTypeName(fieldType.Sel); ok {
				if len(x) != 0 {
					return x + "." + sel, true
				}
				return sel, true
			}
		}
	case *ast.IndexExpr:
		if x, ok := ExprGetFullTypeName(fieldType.X); ok {
			if index, ok := ExprGetFullTypeName(fieldType.Index); ok {
				return x + "[" + index + "]", true
			}
		}
	}

	return "", false
}

func ExprGetCallTypeName(fieldType ast.Expr) (string, bool) {
	switch fieldType := fieldType.(type) {
	case *ast.Ident:
		return fieldType.Name, true
	case *ast.StarExpr:
		return ExprGetFullTypeName(fieldType.X)
	case *ast.ArrayType:
		return ExprGetFullTypeName(fieldType.Elt)
	case *ast.SelectorExpr:
		return ExprGetCallTypeName(fieldType.Sel)
	case *ast.IndexExpr:
		return ExprGetCallTypeName(fieldType.X)
	}

	return "", false
}

func GetFieldDeclTypeName(fieldType ast.Expr) (string, bool) {
	switch fieldType := fieldType.(type) {
	case *ast.Ident:
		return fieldType.Name, true
	case *ast.StarExpr:
		if x, ok := GetFieldDeclTypeName(fieldType.X); ok {
			return "*" + x, true
		}
	case *ast.ArrayType:
		if elt, ok := GetFieldDeclTypeName(fieldType.Elt); ok {
			return "[]" + elt, true
		}
	case *ast.MapType:
		if key, ok := GetFieldDeclTypeName(fieldType.Key); ok {
			if value, ok := GetFieldDeclTypeName(fieldType.Value); ok {
				return "map[" + key + "]" + value, true
			}
		}
	case *ast.SelectorExpr:
		if x, ok := GetFieldDeclTypeName(fieldType.X); ok {
			if sel, ok := GetFieldDeclTypeName(fieldType.Sel); ok {
				if len(x) != 0 {
					return x + "." + sel, true
				}
				return sel, true
			}
		}
	case *ast.IndexExpr:
		if x, ok := GetFieldDeclTypeName(fieldType.X); ok {
			if index, ok := GetFieldDeclTypeName(fieldType.Index); ok {
				return x + "[" + index + "]", true
			}
		}
	case *ast.IndexListExpr:
		if x, ok := GetFieldDeclTypeName(fieldType.X); ok {
			return x + "[" +
				stringsex.JoinSeq(slicesex.MapSeq(fieldType.Indices, func(e ast.Expr) string {
					return gx.ShouldHave(GetFieldDeclTypeName(e))
				}), ", ") +
				"]", true
		}
	case *ast.FuncType:
		return "func (" +
			stringsex.JoinSeq(xiter.Map(
				FuncTypeParamsSeq(fieldType), func(f *ast.Field) string {
					if len(f.Names) > 0 {
						return stringsex.JoinSeq(slicesex.MapSeq(f.Names, func(i *ast.Ident) string { return i.Name }), ", ") +
							" " + gx.ShouldHave(GetFieldDeclTypeName(f.Type))
					} else {
						return gx.ShouldHave(GetFieldDeclTypeName(f.Type))
					}
				}), ", ") + ")", true
	case *ast.Ellipsis:
		if elt, ok := GetFieldDeclTypeName(fieldType.Elt); ok {
			return "..." + elt, true
		}
	}

	return "", false
}

func FuncDeclRecvType(decl *ast.FuncDecl) (ast.Expr, bool) {
	if decl.Recv != nil && len(decl.Recv.List) == 1 {
		return decl.Recv.List[0].Type, true
	}

	return nil, false
}

func FuncTypeParamsSeq(decl *ast.FuncType) iter.Seq[*ast.Field] {
	if decl.Params != nil {
		return func(yield func(*ast.Field) bool) {
			for _, f := range decl.Params.List {
				if !yield(f) {
					return
				}
			}
		}
	}

	return func(yield func(*ast.Field) bool) {}
}

func FuncTypeResultsSeq(decl *ast.FuncType) iter.Seq[*ast.Field] {
	if decl.Results != nil {
		return func(yield func(*ast.Field) bool) {
			for _, f := range decl.Results.List {
				if !yield(f) {
					return
				}
			}
		}
	}

	return func(yield func(*ast.Field) bool) {}
}
