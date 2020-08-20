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
const Title = "assets/title"
const Subtitle = "assets/subtitle"

const Intended = "»"
const NotIntended = "-"

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
		li += Intended
	} else {
		li += NotIntended
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
	subd string
}

func notFound(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Path not found."))
}

func getTree(reqPath string, goal string, files *[]NavbarTreeFile, level int) {
	/* TODO(aosync): Basically make this function a lot better. For example by properly using
	the `path` functions. */
	reqPath, errC := filepath.EvalSymlinks(reqPath)
	goal, errG := filepath.EvalSymlinks(goal)
	if errC != nil || errG != nil {
		return
	}
	filepath.Walk(reqPath, func(p string, info os.FileInfo, err error) error {
		splitted := strings.Split(p, string(os.PathSeparator))
		splitted = splitted[2:]
		sanitizedLink := strings.Join(splitted, string(os.PathSeparator))
		/* The link needs to be sanitized from any pub/subdomain before being used into the links */

		basePath := filepath.Base(p)

		if p == reqPath {
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

		if basePath != View && basePath != Assets {
			navbarElem := NavbarTreeFile{
				path:     sanitizedLink,
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
	getTree(path.Join(Pub, gctx.subd), gctx.path, (*files)[0].subtree, 0) /* path.Join(Pub, Subblog) */
	nav += RenderList(*files)
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
		// TODO(aosync): Use correct path manipulation, notably with path.Join
		if !strings.HasSuffix(reqPath, "/") {
			reqPath += "/"
		}

		viewPath := reqPath + View
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
	/* err := os.Chdir(Pub) // This is bad, I shall get rid of this.
	if err != nil {
		fmt.Println("No pub folder.")
		os.Exit(1)
	} */
	http.HandleFunc("/", handler)
	fmt.Println("Serving localhost")
	http.ListenAndServe(":8080", nil)
}
