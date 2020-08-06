package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const MDSuffix = ".md"
const View = "view.md"

type NavbarTreeFile struct {
	path     string
	base     string
	level    int
	dir      bool
	intended bool
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
				intended: intended,
			}
			*files = append(*files, navbarElem)
			*files = append(*files, *postTree...)
		}

		if info.IsDir() {
			return filepath.SkipDir
		}

		return nil
	})
}

func generateNavbarTree(gctx GenContext) string {
	var nav string

	var files *[]NavbarTreeFile = &[]NavbarTreeFile{}
	getTree(".", gctx.path[2:], files, 0)
	for _, s := range *files {
		nav += "<a href=\"/" + s.path
		if s.dir {
			nav += "/"
		}
		nav += "\">"
		nav += strings.Repeat("&nbsp;", s.level*4)
		if s.intended {
			nav += "+"
		} else {
			nav += "-"
		}
		nav += s.base
		nav += "</a>"
		nav += "<br />"
	}
	return nav
}

func generateASMDHTML(gctx GenContext) string {
	// Temporary
	var code string
	code += "<html>"
	code += generateNavbarTree(gctx)
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

		view := generateASMDHTML(GenContext{
			base: path,
			path: viewPath,
		})
		w.Write([]byte(view))

		return
	} else if strings.HasSuffix(path, MDSuffix) {
		view := generateASMDHTML(GenContext{
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
	http.HandleFunc("/", handler)
	fmt.Println("Serving localhost")
	http.ListenAndServe(":8080", nil)
}
