package website

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func existingUser(db *gorm.DB, id int, username string) (*User, error) {
	user := &User{}
	var result *gorm.DB
	if id > 0 {
		result = db.First(user, id)
	} else {
		result = db.Where("username = ?", username).First(user)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("user not found")
	}
	return user, nil
}

type User struct {
	ID       uint   `gorm:"primaryKey"`
	Username string `gorm:"index"`
	Password []byte
}

func (u *User) updatePassword(pwd string) error {
	hashedPwd, err := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.Password = hashedPwd
	return nil
}

func (u *User) verifyPassword(pwd string) error {
	return bcrypt.CompareHashAndPassword(u.Password, []byte(pwd))
}

func (u *User) exists(db *gorm.DB) bool {
	result := db.Where("username = ?", u.Username).First(&User{})
	return result.RowsAffected > 0
}

func (u *User) create(db *gorm.DB) error {
	result := db.Where("username = ?", u.Username).First(&User{})
	if result.RowsAffected > 0 {
		return fmt.Errorf("user %s already exists", u.Username)
	}
	result = db.Create(u)
	if result.Error != nil {
		return result.Error
	}
	return nil
}

func (u *User) save(db *gorm.DB) error {
	result := db.Model(u).Updates(u)
	return result.Error
}

func (ws *Website) wireUserMgmt() {
	data, err := websiteContent.ReadFile("web/html/usermgmt.html")
	if err != nil {
		panic(err)
	}
	sr := ws.r.Path("/usermgmt/").Methods("GET").Subrouter()
	sr.Use(ws.EnsureAuthMiddleware(AuthMiddlewareConfig{UserNames: []string{"admin"}}))
	sr.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

	userMgmtGETAPI := ws.r.PathPrefix("/usermgmt/users/").Methods("GET").Subrouter()
	userMgmtGETAPI.Use(ws.EnsureAuthMiddleware(AuthMiddlewareConfig{UserNames: []string{"admin"}, IsForAPI: true}))
	userMgmtGETAPI.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		users := []User{}
		result := ws.gdb.Select("id", "username").Find(&users)
		if result.Error != nil {
			http.Error(w, result.Error.Error(), http.StatusInternalServerError)
			return
		}

		userData, err := json.Marshal(users)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/javascript")
		w.Write(userData)
	})

	userMgmtPOSTAPI := ws.r.PathPrefix("/usermgmt/users/").Methods("POST").Subrouter()
	userMgmtPOSTAPI.Use(ws.EnsureAuthMiddleware(AuthMiddlewareConfig{UserNames: []string{"admin"}, IsForAPI: true}))
	userMgmtPOSTAPI.HandleFunc("/create/", func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")
		if username == "" || password == "" {
			log.Println("WARNING: Attempted user creation without username or password")
			http.Error(w, "Cannot create user without details", http.StatusBadRequest)
			return
		}

		u, _ := existingUser(ws.gdb, 0, username)
		if u != nil {
			log.Println("WARNING: Attempted user creation for existing username")
			http.Error(w, "Username already exists", http.StatusBadRequest)
			return
		}

		newUser := User{Username: username}
		if err := newUser.updatePassword(password); err != nil {
			log.Printf("WARNING: error updating password during new user creation: %v", err)
			http.Error(w, "error creating user", http.StatusInternalServerError)
			return
		}
		if err := newUser.create(ws.gdb); err != nil {
			log.Printf("WARNING: error creating user during new user creation: %v", err)
			http.Error(w, "error creating user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	userMgmtPOSTAPI.HandleFunc("/save/", func(w http.ResponseWriter, r *http.Request) {
		uidStr := r.FormValue("uid")
		password := r.FormValue("pwd")
		if uidStr == "" || password == "" {
			http.Error(w, "empty user ID or password", http.StatusBadRequest)
			return
		}
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			log.Printf("error in user save when handling uid: %v", err)
			http.Error(w, "bad user ID", http.StatusBadRequest)
			return
		}

		u, err := existingUser(ws.gdb, uid, "")
		if err != nil {
			log.Printf("error in user save: user (%d) doesn't exist", uid)
			http.Error(w, "user doesn't exist", http.StatusBadRequest)
			return
		}
		if err := u.updatePassword(password); err != nil {
			log.Printf("error in user save updating password: %v", err)
			http.Error(w, "unable to save", http.StatusInternalServerError)
			return
		}
		if err := u.save(ws.gdb); err != nil {
			log.Printf("error in user save while saving: %v", err)
			http.Error(w, "unable to save", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
	userMgmtPOSTAPI.HandleFunc("/delete/", func(w http.ResponseWriter, r *http.Request) {
		uidStr := r.FormValue("uid")
		if uidStr == "" {
			http.Error(w, "empty user ID", http.StatusBadRequest)
			return
		}
		uid, err := strconv.Atoi(uidStr)
		if err != nil {
			log.Printf("error in user delete when handling uid: %v", err)
			http.Error(w, "bad user ID", http.StatusBadRequest)
			return
		}

		u, err := existingUser(ws.gdb, uid, "")
		if err != nil {
			log.Printf("error in user delete: user (%d) doesn't exist", uid)
			http.Error(w, "user doesn't exist", http.StatusBadRequest)
			return
		}
		result := ws.gdb.Delete(u)
		if result.Error != nil {
			log.Printf("error deleting user: %v", err)
			http.Error(w, "error deleting user", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte("{}"))
	})
}
