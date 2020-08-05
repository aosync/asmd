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

func generateASMDHTML(dirPath string, filename string) string {
	var res string
	res += "<html>"
	res += "Directory: " + dirPath + "<br />"
	res += "File: " + filename + "<br />"
	var files *[]NavbarTreeFile = &[]NavbarTreeFile{}
	getTree(".", filename[2:], files, 0)
	for _, s := range *files {
		res += "<a href=\"/" + s.path
		if s.dir {
			res += "/"
		}
		res += "\">"
		res += strings.Repeat("&nbsp;", s.level*4)
		if s.intended {
			res += "+"
		} else {
			res += "-"
		}
		res += s.base
		if s.dir {
			res += "/"
		}
		res += "</a>"
		res += "<br />"
	}
	res += "</html>"
	return res
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

			// If required path is not found, retry with `.md` appended
			goto RetryMD
		}
		// Otherwise, path not found.
		notFound(w, r)
		return
	}

	if f.IsDir() {
		//	If the path is a directory, I should check if view.md exists in this directory. If it doesn't, then the path is invalid.
		viewPath := path + View
		_, err := os.Stat(viewPath)
		if err != nil {
			notFound(w, r)
			return
		}

		// If it does, generate the HTML for a beautiful view :^)
		view := generateASMDHTML(path, viewPath)
		w.Write([]byte(view))
		return
	} else if retried || strings.HasSuffix(path, MDSuffix) {
		// If the path is a MD file, serve it within the view too, because I don't want all pages to actually be folders.
		view := generateASMDHTML(filepath.Dir(path), path)
		w.Write([]byte(view))
		return
	}

	// If the path is anything else, serve it raw.
	contents, _ := ioutil.ReadFile(path)
	w.Write(contents)
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Serving localhost")
	http.ListenAndServe(":8080", nil)
}
