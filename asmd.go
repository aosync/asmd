package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
)

const MDSuffix = ".md"
const View = "view.md"

const DefaultSubd = "default"

const Assets = "assets"
const Pub = "pub"

const Style = "assets/style.css"

type NavbarTreeFile struct {
	link     string
	base     string
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
	li += "<a href=\"/" + v.link + "\">"
	if v.intended {
		li += "<span class=\"nav-intended\"></span>"
	} else {
		li += "<span class=\"nav-not-intended\"></span>"
	}
	li += v.base
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
	subd string
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Path not found."))
}

func removeMountpoint(requestedPath string) string {
	sep := string(os.PathSeparator)
	splitted := strings.Split(requestedPath, sep)
	splitted = splitted[2:]
	return strings.Join(splitted, sep)
}

func pathStartsWith(path string, start string) bool {
	return strings.HasPrefix(path+"/", start+"/")
}

func getTree(reqPath string, goal string, files *[]NavbarTreeFile) {
	/* Symlinks must be evalled for them to work in filepath.Walk */
	reqPath, errC := filepath.EvalSymlinks(reqPath)
	goal, errG := filepath.EvalSymlinks(goal)
	if errC != nil || errG != nil {
		return
	}

	filepath.Walk(reqPath, func(current string, info os.FileInfo, err error) error {
		if current == reqPath {
			/* Ignore current folder of iteration */
			return nil
		}

		dir := info.IsDir()
		/* The link needs to be sanitized from any pub/subdomain before being used into the links */
		link := removeMountpoint(current)
		basePath := filepath.Base(current)

		if basePath != View && basePath != Assets {
			subtree := &[]NavbarTreeFile{}
			intended := false

			if pathStartsWith(goal, current) {
				intended = true
				if dir {
					getTree(current, goal, subtree)
				}
			}
			if dir {
				basePath += "/"
			}
			element := NavbarTreeFile{
				link:     link,
				base:     basePath,
				subtree:  subtree,
				intended: intended,
			}
			*files = append(*files, element)
		}

		if dir {
			return filepath.SkipDir
		}

		return nil
	})
}

func RenderNavbar(gctx GenContext) string {
	var nav string
	nav += "<nav>"
	begin := NavbarTreeFile{
		link:     "",
		base:     "/",
		subtree:  &[]NavbarTreeFile{},
		intended: true,
	}
	getTree(path.Join(Pub, gctx.subd), gctx.path, begin.subtree)
	tree := &[]NavbarTreeFile{begin}
	nav += RenderList(*tree)
	nav += "</nav>"
	return nav
}

func RenderArticle(gctx GenContext) string {
	var article string
	article += "<article class=\"rest\">"

	art, err := ioutil.ReadFile(gctx.path)
	if err != nil {
		return ""
	}

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	md := art
	html := markdown.ToHTML(md, nil, renderer)

	article += string(html)
	article += "</article>"

	return article
}

func RenderBody(gctx GenContext) string {
	var body string
	body += "<body>"
	body += "<header><a href=\"/\"><h1 id=\"title\"></h1><h3 id=\"subtitle\"></h3></a></header>"
	body += RenderNavbar(gctx)
	body += RenderArticle(gctx)
	body += "</body>"
	return body
}

func RenderStyle(gctx GenContext) string {
	var style string

	style += "<style rel=\"stylesheet\">"

	css, err := ioutil.ReadFile(path.Join(Pub, gctx.subd, Style))
	if err != nil {
		return ""
	}

	style += string(css)
	style += "</style>"

	return style
}

func RenderPage(gctx GenContext) string {
	var html string
	html += "<!doctype html>"
	html += "<html>"
	html += "<title>" + gctx.subd + "</title>"
	html += RenderStyle(gctx)
	html += RenderBody(gctx)
	html += "</html>"
	return html
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Received something.") // For debug purposes

	subd := strings.Split(r.Host, ".")[0]
	_, err := os.Stat(path.Join(Pub, subd))
	if err != nil {
		subd = DefaultSubd
	}
	reqPath := r.URL.Path[1:]
	reqPath = path.Clean(reqPath)
	reqPath = path.Join(Pub, subd, reqPath) /* (Pub, subd, reqPath) */

	retried := false

RetryMD:
	f, err := os.Stat(reqPath)

	if err != nil {
		if !retried {
			reqPath = reqPath + MDSuffix
			retried = true

			goto RetryMD
		}

		notFound(w, r)
		return
	}

	if f.IsDir() {
		viewPath := path.Join(reqPath, View)
		_, err := os.Stat(viewPath)

		if err != nil {
			notFound(w, r)
			return
		}

		view := RenderPage(GenContext{
			base: reqPath,
			path: viewPath,
			subd: subd,
		})
		w.Write([]byte(view))

		return
	} else if strings.HasSuffix(reqPath, MDSuffix) {
		view := RenderPage(GenContext{
			base: filepath.Dir(reqPath),
			path: reqPath,
			subd: subd,
		})
		w.Write([]byte(view))

		return
	}

	contents, _ := ioutil.ReadFile(reqPath)
	w.Write(contents)
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Serving localhost")
	http.ListenAndServe(":8080", nil)
}
