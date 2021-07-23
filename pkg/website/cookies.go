package website

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/securecookie"
	"gorm.io/gorm"
)

type Key struct {
	ID       uint `gorm:"primaryKey"`
	HashKey  []byte
	BlockKey []byte
}

func key(db *gorm.DB) (Key, error) {
	k := Key{}
	result := db.First(&k, 1)
	if result.RowsAffected == 0 {
		k.ID = 1
		k.HashKey = securecookie.GenerateRandomKey(64)
		k.BlockKey = securecookie.GenerateRandomKey(32)
		result = db.Create(k)
		if result.Error != nil {
			return k, result.Error
		}
	}
	result = db.First(&k, 1)
	if result.RowsAffected == 0 {
		return k, fmt.Errorf("key could not be created")
	}
	return k, nil
}

var cookies *cookieHandler

func wireCookies(db *gorm.DB, appName string, timeout time.Duration) error {
	if db == nil {
		return nil
	}
	var oerr error
	once := sync.Once{}
	once.Do(func() {
		k, err := key(db)
		if err != nil {
			oerr = err
			return
		}
		to := timeout
		if to == 0 {
			to = time.Minute * 5
		}
		s := securecookie.New(k.HashKey, k.BlockKey)
		ck := &cookieHandler{
			SecureCookie: s,
			cookieName:   fmt.Sprintf("%s_encrypted_session", appName),
			timeout:      to,
		}
		cookies = ck
	})
	return oerr
}

type cookiekey int

const mycookieKey = cookiekey(10)

type CookieData struct {
	UserID    int
	VisitorID string
	CreatedAt time.Time
	UpdatedAt time.Time
	Extras    map[string]string

	user *User
}

func (cd *CookieData) evaluateUser(gdb *gorm.DB, timeout time.Duration) {
	if cd.UserID == 0 {
		return
	}
	if time.Since(cd.UpdatedAt) > timeout {
		log.Printf("User: %d cookie timed out [%v]", cd.UserID, time.Since(cd.UpdatedAt))
		cd.UserID = 0
		return
	}
	u, err := existingUser(gdb, int(cd.UserID), "")
	if err != nil {
		log.Printf("User: %d was logged in but doesn't exist anymore", cd.UserID)
		cd.UserID = 0
		return
	}
	cd.user = u
}

func (cd *CookieData) IsLoggedIn(timeout time.Duration) bool {
	if cd.UserID < 1 {
		return false
	}
	if time.Since(cd.UpdatedAt) > timeout {
		log.Printf("User: %d cookie timed out [%v]", cd.UserID, time.Since(cd.UpdatedAt))
		return false
	}
	log.Printf("User: %d cookie timer: [%v]", cd.UserID, time.Since(cd.UpdatedAt))
	return true
}

type cookieHandler struct {
	*securecookie.SecureCookie
	cookieName string
	timeout    time.Duration
}

func (ck *cookieHandler) read(r *http.Request) *CookieData {
	c, err := r.Cookie(ck.cookieName)
	if err != nil {
		return nil
	}
	value := &CookieData{}
	if err := ck.Decode(ck.cookieName, c.Value, value); err != nil {
		return nil
	}
	if value.Extras == nil {
		value.Extras = make(map[string]string)
	}
	return value
}

func (ck *cookieHandler) readTransient(r *http.Request) *CookieData {
	return r.Context().Value(mycookieKey).(*CookieData)
}

func (ck *cookieHandler) write(w http.ResponseWriter, value *CookieData) error {
	encoded, err := ck.Encode(ck.cookieName, value)
	if err != nil {
		return err
	}
	cookie := &http.Cookie{
		Name:  ck.cookieName,
		Value: encoded,
		Path:  "/",
	}
	http.SetCookie(w, cookie)
	return nil
}

func (ck *cookieHandler) new() *CookieData {
	return &CookieData{
		VisitorID: uuid.NewString(),
		CreatedAt: time.Now(),
		Extras:    make(map[string]string),
	}
}
