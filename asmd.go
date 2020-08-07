package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
)

const MDSuffix = ".md"
const View = "view.md"
const Pub = "pub"

type NavbarTreeFile struct {
	path     string
	base     string
	level    int
	dir      bool
	subtree  *[]NavbarTreeFile
	intended bool
}

func RenderList(elems []NavbarTreeFile) string {
	var ul string
	ul += "<ul>"
	for _, s := range elems {
		ul += s.Render()
	}
	ul += "</ul>"
	return ul
}

func (v NavbarTreeFile) Render() string {
	var li string
	li += "<li>"
	li += "<a href=\"/" + v.path + "\">"
	if v.intended {
		li += "+"
	} else {
		li += "-"
	}
	li += v.base
	if v.dir {
		li += "/"
	}
	li += "</a>"
	li += "</li>"
	if len(*v.subtree) > 0 {
		li += "<li>"
		li += RenderList(*v.subtree)
		li += "</li>"
	}
	return li
}

type GenContext struct {
	base string
	path string
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Path not found."))
}

func getTree(path string, goal string, files *[]NavbarTreeFile, level int) {
	filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		basePath := filepath.Base(p)

		if p == path {
			return nil
		}

		var postTree *[]NavbarTreeFile = &[]NavbarTreeFile{}
		intended := false

		// TODO(aosync): There is certainly a way to improve this.
		if strings.HasPrefix(goal+"/", p+"/") {
			intended = true
			if info.IsDir() {
				getTree(p, goal, postTree, level+1)
			}
		}

		if basePath != View {
			navbarElem := NavbarTreeFile{
				path:     p,
				base:     basePath,
				level:    level,
				dir:      info.IsDir(),
				subtree:  postTree,
				intended: intended,
			}
			*files = append(*files, navbarElem)
		}

		if info.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})
}

func RenderNavbar(gctx GenContext) string {
	var nav string
	nav += "<nav>"
	var files *[]NavbarTreeFile = &[]NavbarTreeFile{}
	*files = append(*files, NavbarTreeFile{
		path:     "",
		base:     "",
		level:    0,
		dir:      true,
		subtree:  &[]NavbarTreeFile{},
		intended: true,
	})
	getTree(".", gctx.path[2:], (*files)[0].subtree, 0)
	nav += RenderList(*files)
	nav += "</nav>"
	return nav
}

func RenderBody(gctx GenContext) string {
	var body string
	body += "<body>"
	body += RenderNavbar(gctx)
	body += "<article class=\"rest\">"
	article, _ := ioutil.ReadFile(gctx.path)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	md := article
	html := markdown.ToHTML(md, nil, renderer)
	body += string(html)
	body += "</article>"
	body += "</body>"
	return body
}

func RenderPage(gctx GenContext) string {
	// Temporary
	var code string
	code += "<!doctype html>"
	code += "<html>"
	code += "<style rel=\"stylesheet\">"
	style, _ := ioutil.ReadFile("style.css")
	code += string(style)
	code += "</style>"
	code += RenderBody(gctx)
	code += "</html>"
	return code
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received something.") // For debug purposes

	path := "." + r.URL.Path
	retried := false

RetryMD:
	f, err := os.Stat(path)

	if err != nil {
		if !retried {
			path = path + MDSuffix
			retried = true

			goto RetryMD
		}

		notFound(w, r)
		return
	}

	if f.IsDir() {
		// TODO(aosync): Use correct path manipulation, notably with path.Join
		if !strings.HasSuffix(path, "/") {
			path += "/"
		}

		viewPath := path + View
		_, err := os.Stat(viewPath)

		if err != nil {
			notFound(w, r)
			return
		}

		view := RenderPage(GenContext{
			base: path,
			path: viewPath,
		})
		w.Write([]byte(view))

		return
	} else if strings.HasSuffix(path, MDSuffix) {
		view := RenderPage(GenContext{
			base: filepath.Dir(path),
			path: path,
		})
		w.Write([]byte(view))

		return
	}

	contents, _ := ioutil.ReadFile(path)
	w.Write(contents)
}

func main() {
	err := os.Chdir(Pub)
	if err != nil {
		fmt.Println("No pub folder.")
		os.Exit(1)
	}
	http.HandleFunc("/", handler)
	fmt.Println("Serving localhost")
	http.ListenAndServe(":8080", nil)
}
