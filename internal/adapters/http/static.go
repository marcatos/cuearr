package httpapi

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/marcatos/cuearr/internal/adapters/auth"
	"github.com/marcatos/cuearr/web"
)

func newStaticHandler() http.Handler {
	sub, err := fs.Sub(web.Content, ".")
	if err != nil {
		panic("web embed fs: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" || auth.IsLoginStaticPath(r.URL.Path) || !strings.Contains(path, ".") {
			http.ServeFileFS(w, r, sub, "index.html")
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
