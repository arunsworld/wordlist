package website

import (
	"encoding/json"
	"log"
	"net/http"
)

func (ws *Website) EnableLogin() error {
	if ws.gdb == nil {
		return nil
	}
	if err := ws.gdb.AutoMigrate(&User{}); err != nil {
		return err
	}
	if err := ws.gdb.AutoMigrate(&Key{}); err != nil {
		return err
	}
	if err := wireCookies(ws.gdb, ws.appName, 0); err != nil {
		return err
	}
	adminUser := &User{
		Username: "admin",
	}
	if !adminUser.exists(ws.gdb) {
		log.Printf("admin user doesn't exist... going to create new...")
		adminUser.updatePassword("admin")
		if err := adminUser.create(ws.gdb); err != nil {
			return err
		}
	}
	ws.r.Use(ws.cookiesMiddleware())
	ws.wireLoginPage()
	ws.wireLogout()
	ws.wireUserMgmt()
	return nil
}

func (ws *Website) wireLoginPage() {
	data, err := websiteContent.ReadFile("web/html/login.html")
	if err != nil {
		panic(err)
	}
	sr := ws.r.Path("/login/").Subrouter()
	sr.Methods("GET").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ws.IsRequestAuthenticated(r) {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})
	sr.Methods("POST").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ws.handleLoginAttempt(w, r)
	})
}

func (ws *Website) handleLoginAttempt(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")
	if username == "" || password == "" {
		log.Println("WARNING: Login POST without username or password")
		http.Redirect(w, r, "/login/", http.StatusTemporaryRedirect)
		return
	}

	user, err := existingUser(ws.gdb, 0, username)
	if err != nil {
		log.Printf("WARNING: Login attempt for %s that doesn't exist", username)
		http.Error(w, "Username & Password not recognized", http.StatusUnauthorized)
		return
	}

	if err := user.verifyPassword(password); err != nil {
		log.Printf("WARNING: Login attempt for %s with invalid credentials", username)
		http.Error(w, "Username & Password not recognized", http.StatusUnauthorized)
		return
	}

	cd := cookies.readTransient(r)
	nextURI := cd.Extras["next"]
	if nextURI == "" {
		nextURI = "/"
	}
	cd.UserID = int(user.ID)
	cd.Extras["next"] = ""

	if err := cookies.write(w, cd); err != nil {
		log.Printf("ERROR writing cookie: %v", err)
		http.Error(w, "Problem logging in", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	v := struct {
		NextURI string
	}{
		NextURI: nextURI,
	}
	json.NewEncoder(w).Encode(v)
}

func (ws *Website) wireLogout() {
	ws.r.HandleFunc("/logout/", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			http.Redirect(w, r, "/login/", http.StatusTemporaryRedirect)
		}()
		cd := cookies.readTransient(r)
		cd.UserID = 0
		cd.Extras = nil
		if err := cookies.write(w, cd); err != nil {
			log.Printf("ERROR writing cookie (logout): %v", err)
		}
	})
}
