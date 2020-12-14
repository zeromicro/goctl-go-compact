package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/parser"
	"go/token"
	"html/template"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"unicode"

	"github.com/tal-tech/go-zero/core/stringx"
	"github.com/tal-tech/go-zero/tools/goctl/api/gogen"
	"github.com/tal-tech/go-zero/tools/goctl/api/spec"
	"github.com/tal-tech/go-zero/tools/goctl/api/util"
	"github.com/tal-tech/go-zero/tools/goctl/plugin"
	util2 "github.com/tal-tech/go-zero/tools/goctl/util"
	"github.com/tal-tech/go-zero/tools/goctl/util/ctx"
	"github.com/tal-tech/go-zero/tools/goctl/util/format"
	"go.etcd.io/etcd/pkg/fileutil"
)

const (
	groupProperty        = "group"
	interval             = "internal/"
	handlerDir           = interval + "handler"
	logicDir             = interval + "logic"
	typesPacket          = "types"
	typesDir             = interval + typesPacket
	contextDir           = interval + "svc"
	pkgSep               = "/"
	projectOpenSourceUrl = "github.com/tal-tech/go-zero"
	handlerImports       = `package handler

import (
	"net/http"

	{{.packages}}
)
`
	handlerTemplate = `func {{.HandlerName}}(ctx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		{{if .HasRequest}}var req types.{{.RequestType}}
		if err := httpx.Parse(r, &req); err != nil {
			httpx.Error(w, err)
			return
		}{{end}}

		l := logic.New{{.LogicType}}(r.Context(), ctx)
		{{if .HasResp}}resp, {{end}}err := l.{{.Call}}({{if .HasRequest}}req{{end}})
		if err != nil {
			httpx.Error(w, err)
		} else {
			{{if .HasResp}}httpx.OkJson(w, resp){{else}}httpx.Ok(w){{end}}
		}
	}
}`
)

type Handler struct {
	HandlerName string
	RequestType string
	LogicType   string
	Call        string
	HasResp     bool
	HasRequest  bool
}

func main() {
	plugin, err := plugin.NewPlugin()
	if err != nil {
		panic(err)
	}

	err = gogen.DoGenProject(plugin.ApiFilePath, plugin.Dir, plugin.Style)
	if err != nil {
		panic(err)
	}

	var api = plugin.Api
	for _, group := range api.Service.Groups {
		if len(group.Routes) == 0 {
			continue
		}

		route := group.Routes[0]
		folder := getHandlerFolderPath(group, route)
		for _, route := range group.Routes {
			filename, err := format.FileNamingFormat(plugin.Style, getHandlerName(route, folder))
			if err != nil {
				panic(err)
			}

			filename = filename + ".go"
			os.Remove(filepath.Join(plugin.Dir, getHandlerFolderPath(group, route), filename))
		}

		err := gen(folder, group, plugin.Dir, plugin.Style)
		if err != nil {
			debug.PrintStack()
			panic(err)
		}
	}
}

func gen(folder string, group spec.Group, dir, nameStyle string) error {
	var routes = group.Routes
	parentPkg, err := getParentPackage(dir)
	if err != nil {
		return err
	}

	filename, err := format.FileNamingFormat(nameStyle, "handlers")
	if err != nil {
		return err
	}

	filename = filename + ".go"
	filename = filepath.Join(dir, folder, filename)
	var fp *os.File

	var hasExist = true
	if !fileutil.Exist(filename) {
		_, err = os.Create(filename)
		if err != nil {
			return err
		}
		hasExist = false
	}

	fp, err = os.OpenFile(filename, os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer fp.Close()

	text, err := util2.LoadTemplate("api", "handlers.tpl", handlerTemplate)
	if err != nil {
		return err
	}

	var funcs []string
	var imports []string
	for _, route := range routes {
		var handler = getHandlerName(route, folder)
		if hasExist && funcExist(filename, handler) {
			continue
		}
		fmt.Println("merge handler " + handler)

		handleObj := Handler{
			HandlerName: handler,
			RequestType: strings.Title(route.RequestType.Name),
			LogicType:   strings.Title(getLogicName(route)),
			Call:        strings.Title(strings.TrimSuffix(handler, "Handler")),
			HasResp:     len(route.ResponseType.Name) > 0,
			HasRequest:  len(route.RequestType.Name) > 0,
		}

		buffer := new(bytes.Buffer)
		err = template.Must(template.New("handlerTemplate").Parse(text)).Execute(buffer, handleObj)
		if err != nil {
			return err
		}

		funcs = append(funcs, buffer.String())

		for _, item := range genHandlerImports(group, route, parentPkg) {
			if !stringx.Contains(imports, item) {
				imports = append(imports, item)
			}
		}
	}

	buffer := new(bytes.Buffer)
	if !hasExist {
		importsStr := strings.Join(imports, "\n\t")
		err = template.Must(template.New("handlerImports").Parse(handlerImports)).Execute(buffer, map[string]string{
			"packages": importsStr,
		})
		if err != nil {
			return err
		}
	}

	formatCode := formatCode(strings.ReplaceAll(buffer.String(), "&#34;", "\"") + strings.Join(funcs, "\n\n"))
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	if len(content) > 0 {
		formatCode = string(content) + "\n" + formatCode
	}
	_, err = fp.WriteString(formatCode)
	return err
}

func funcExist(filename, funcName string) bool {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return false
	}

	set := token.NewFileSet()
	packs, err := parser.ParseFile(set, filename, string(data), parser.ParseComments)
	if err != nil {
		panic(err)
	}

	for _, d := range packs.Decls {
		if fn, isFn := d.(*ast.FuncDecl); isFn {
			if fn.Name.String() == funcName {
				return true
			}
		}
	}
	return false
}

func getHandlerName(route spec.Route, folder string) string {
	handler, err := getHandlerBaseName(route)
	if err != nil {
		panic(err)
	}

	handler = handler + "Handler"
	if folder != handlerDir {
		handler = strings.Title(handler)
	}
	return handler
}

func getHandlerFolderPath(group spec.Group, route spec.Route) string {
	folder, ok := util.GetAnnotationValue(route.Annotations, "server", groupProperty)
	if !ok {
		folder, ok = util.GetAnnotationValue(group.Annotations, "server", groupProperty)
		if !ok {
			return handlerDir
		}
	}
	folder = strings.TrimPrefix(folder, "/")
	folder = strings.TrimSuffix(folder, "/")
	return path.Join(handlerDir, folder)
}

func getHandlerBaseName(route spec.Route) (string, error) {
	handler, ok := util.GetAnnotationValue(route.Annotations, "server", "handler")
	if !ok {
		return "", fmt.Errorf("missing handler annotation for %q", route.Path)
	}

	for _, char := range handler {
		if !unicode.IsDigit(char) && !unicode.IsLetter(char) {
			return "", errors.New(fmt.Sprintf("route [%s] handler [%s] invalid, handler name should only contains letter or digit",
				route.Path, handler))
		}
	}

	handler = strings.TrimSpace(handler)
	handler = strings.TrimSuffix(handler, "handler")
	handler = strings.TrimSuffix(handler, "Handler")
	return handler, nil
}

func getLogicName(route spec.Route) string {
	handler, err := getHandlerBaseName(route)
	if err != nil {
		panic(err)
	}

	return handler + "Logic"
}

func formatCode(code string) string {
	ret, err := goformat.Source([]byte(code))
	if err != nil {
		return code
	}

	return string(ret)
}

func genHandlerImports(group spec.Group, route spec.Route, parentPkg string) []string {
	var imports []string
	imports = append(imports, fmt.Sprintf("\"%s\"",
		joinPackages(parentPkg, getLogicFolderPath(group, route))))
	imports = append(imports, fmt.Sprintf("\"%s\"", joinPackages(parentPkg, contextDir)))
	if len(route.RequestType.Name) > 0 {
		imports = append(imports, fmt.Sprintf("\"%s\"\n", joinPackages(parentPkg, typesDir)))
	}
	imports = append(imports, fmt.Sprintf("\"%s/rest/httpx\"", projectOpenSourceUrl))

	return imports
}

func joinPackages(pkgs ...string) string {
	return strings.Join(pkgs, pkgSep)
}

func getLogicFolderPath(group spec.Group, route spec.Route) string {
	folder, ok := util.GetAnnotationValue(route.Annotations, "server", groupProperty)
	if !ok {
		folder, ok = util.GetAnnotationValue(group.Annotations, "server", groupProperty)
		if !ok {
			return logicDir
		}
	}
	folder = strings.TrimPrefix(folder, "/")
	folder = strings.TrimSuffix(folder, "/")
	return path.Join(logicDir, folder)
}

func getParentPackage(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	projectCtx, err := ctx.Prepare(abs)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(filepath.Join(projectCtx.Path, strings.TrimPrefix(projectCtx.WorkDir, projectCtx.Dir))), nil
}
