package website

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/unrolled/logger"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

//go:embed web/*
var websiteContent embed.FS

func NewWebsite(dbName, appName string) *Website {
	result := &Website{
		r:             mux.NewRouter(),
		cookieName:    fmt.Sprintf("%s_session", appName),
		appName:       appName,
		cookieTimeout: time.Minute * 5,
	}
	result.addLoggingMiddleware()
	if dbName != "" {
		_gdb, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
		if err != nil {
			panic("failed to connect database")
		}
		_db, err := _gdb.DB()
		if err != nil {
			panic("failed to connect database")
		}
		result.gdb = _gdb
		result.db = _db
	}
	return result
}

type Website struct {
	r             *mux.Router
	gdb           *gorm.DB
	db            *sql.DB
	appName       string
	cookieName    string
	cookieTimeout time.Duration
}

func (ws *Website) Router() *mux.Router {
	return ws.r
}

func (ws *Website) DB() *gorm.DB {
	return ws.gdb
}

func (ws *Website) EnableStatic() {
	ws.r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(mustFSSub(websiteContent, "web/static")))))
}

func (ws *Website) Serve(port int) error {
	ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	return ws.ServeWithCtx(ctx, port)
}

func (ws *Website) ServeWithCtx(ctx context.Context, port int) error {
	addr := fmt.Sprintf(":%d", port)
	srv := &http.Server{
		Addr:    addr,
		Handler: ws.r,
	}
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server start error: %v", err)
			errCh <- err
		}
	}()
	time.Sleep(time.Second)
	log.Printf("Serving on URL: http://localhost:%d/", port)
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("initiating graceful shutdown of server...")
		ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctxShutDown); err != nil {
			log.Printf("error during graceful shutdown: %v", err)
		}
		return nil
	}
}

func (ws *Website) Close() error {
	if ws.db == nil {
		return nil
	}
	return ws.db.Close()
}

func (*Website) IsRequestAuthenticated(r *http.Request) bool {
	cd := cookies.readTransient(r)
	return cd.UserID > 0
}

func (*Website) AuthenticatedUser(r *http.Request) *User {
	cd := cookies.readTransient(r)
	return cd.user
}

type AuthMiddlewareConfig struct {
	UserIDs   []int
	UserNames []string
	IsForAPI  bool
}

func (ws *Website) EnsureAuthMiddleware(config AuthMiddlewareConfig) mux.MiddlewareFunc {
	notAuthorized, err := websiteContent.ReadFile("web/html/not-authorized.html")
	if err != nil {
		panic(err)
	}
	var userIDFilter map[int]struct{}
	if len(config.UserIDs) > 0 {
		userIDFilter = make(map[int]struct{})
		for _, uid := range config.UserIDs {
			userIDFilter[uid] = struct{}{}
		}
	}
	var userNameFilter map[string]struct{}
	if len(config.UserNames) > 0 {
		userNameFilter = make(map[string]struct{})
		for _, username := range config.UserNames {
			userNameFilter[username] = struct{}{}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := ws.AuthenticatedUser(r)
			if user == nil {
				if config.IsForAPI {
					http.Error(w, "not authorized", http.StatusForbidden)
				} else {
					data := map[string]string{
						"next": r.RequestURI,
					}
					ws.StoreInCookie(w, r, data)
					http.Redirect(w, r, "/login/", http.StatusTemporaryRedirect)
				}
				return
			}
			userIsAuthenticated := true
			if userIDFilter != nil {
				_, ok := userIDFilter[int(user.ID)]
				if !ok {
					userIsAuthenticated = false
				}
			}
			if userNameFilter != nil {
				_, ok := userNameFilter[user.Username]
				if !ok {
					userIsAuthenticated = false
				}
			}
			if !userIsAuthenticated {
				if config.IsForAPI {
					http.Error(w, "not authorized", http.StatusForbidden)
				} else {
					w.Header().Set("Content-Type", "text/html")
					w.Write(notAuthorized)
				}
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (ws *Website) StoreInCookie(w http.ResponseWriter, r *http.Request, data map[string]string) error {
	cd := cookies.readTransient(r)
	for k, v := range data {
		cd.Extras[k] = v
	}
	if err := cookies.write(w, cd); err != nil {
		return err
	}
	return nil
}

func (ws *Website) addLoggingMiddleware() {
	l := logger.New()
	ws.r.Use(func(next http.Handler) http.Handler {
		return l.Handler(next)
	})
}

func (ws *Website) cookiesMiddleware() mux.MiddlewareFunc {
	if cookies == nil {
		panic("cookie handler not wired, but middleware requested...")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cd := cookies.read(r)
			if cd == nil {
				cd = cookies.new()
			} else {
				cd.evaluateUser(ws.gdb, ws.cookieTimeout)
			}
			cd.UpdatedAt = time.Now()
			if err := cookies.write(w, cd); err != nil {
				log.Printf("ERROR writing cookie: %v", err)
			}
			ctx := context.WithValue(r.Context(), mycookieKey, cd)
			r = r.WithContext(ctx)
			next.ServeHTTP(w, r)
		})
	}
}

func mustFSSub(src fs.FS, dir string) fs.FS {
	fsys, err := fs.Sub(src, dir)
	if err != nil {
		panic(err)
	}
	return fsys
}
