# Wordlist webapp

* Deployed at words.iapps365.com
* To build: make build

## Documentation

* https://gorm.io/docs/
* https://getbootstrap.com/docs/5.0/getting-started/introduction/
* https://vuejs.org/v2/guide/instance.html
* https://github.com/gorilla/mux

## Typical server-side patterns

### Serving static webpage

    data, err := webContent.ReadFile("web/html/xxx.html")
	if err != nil {
		panic(err)
	}

    xxx := ws.Router().Path("/xxx/").Methods("GET").Subrouter()
	xxx.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{}))
	xxx.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write(data)
	})

### APIs (GET)

    xxxAPI := ws.Router().PathPrefix("/xxx-api/").Methods("GET").Subrouter()
	xxxAPI.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{IsForAPI: true}))

    xxxAPI.HandleFunc("/do-something/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		if err := json.NewEncoder(w).Encode(data); err != nil {
			log.Printf("error marshaling data as JSON: %v", err)
			http.Error(w, "unable to do something", http.StatusInternalServerError)
			return
		}
	})

### APIs (POST - Form based)

    xxxAPI := ws.Router().PathPrefix("/xxx-api/").Methods("POST").Subrouter()
	xxxAPI.Use(ws.EnsureAuthMiddleware(website.AuthMiddlewareConfig{IsForAPI: true}))

    xxxAPI.HandleFunc("/do-something/", func(w http.ResponseWriter, r *http.Request) {
		param1 := r.FormValue("param1")
        param2 := r.FormValue("param2")
        ...
	})

### APIs (POST - JSON)

    err := json.NewDecoder(r.Body).Decode(&data)
    if err != nil {
        log.Printf("error decoding answers as JSON: %v", err)
        http.Error(w, "error doing something", http.StatusInternalServerError)
        return
    }