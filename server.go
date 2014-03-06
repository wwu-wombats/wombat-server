package main

import (
    "fmt"
    "net/http"
    "html/template"
    "encoding/json"
    "time"
    "io/ioutil"
    "os"
    "path/filepath"
    "github.com/gorilla/mux"
    "github.com/gorilla/context"
    "github.com/gorilla/securecookie"
    "github.com/Wombats/goauth"
)

type SiteData struct {
    Root string
}

var (
    apiroot string = "/api"
    ctx SiteData = SiteData{Root: "localhost"}
    tpls = template.Must(template.ParseFiles("tpls/login.html"))
    fileroot string = "files"
    aaa goauth.Authorizer = goauth.NewAuthorizer("data/auth", "wombat-salt", securecookie.GenerateRandomKey(32))
)

func main() {
    fmt.Println("Starting server on port 8080.")
    r := mux.NewRouter()

    // Pages
    r.HandleFunc("/login", getLogin).Methods("GET")
    r.HandleFunc("/login", postLogin).Methods("POST")
    r.HandleFunc("/register", postRegister).Methods("POST")
    r.HandleFunc("/logout", logout)

    // Static files
    http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))

    // API
    r.HandleFunc(apiroot, addSlash);
    r.HandleFunc(apiroot + "/",                     authorize(handleApiRoot)).Methods("GET")
    r.HandleFunc(apiroot + "/create/{path:.*}",     authorize(jsonResponse(handleApiCreate))).Methods("POST")
    r.HandleFunc(apiroot + "/move",                 authorize(jsonResponse(handleApiMove))).Methods("POST")
    r.HandleFunc(apiroot + "/delete/{path:.*}",     authorize(jsonResponse(handleApiRemove))).Methods("POST")
    r.HandleFunc(apiroot + "/modify/{path:.*}",     authorize(jsonResponse(handleApiModify))).Methods("POST")
    r.HandleFunc(apiroot + "/download/{path:.*}",   authorize(handleApiDownload)).Methods("GET")
    r.HandleFunc(apiroot + "/list", addSlash);
    r.HandleFunc(apiroot + "/list/{path:.*}",       authorize(handleApiList)).Methods("GET")
    r.HandleFunc(apiroot + "/tree", addSlash);
    r.HandleFunc(apiroot + "/tree/{path:.*}",       authorize(handleApiTree)).Methods("GET")

    r.StrictSlash(false)

    http.Handle("/", r)
    http.ListenAndServe(":8080", nil)
}

type JsonString map[string]interface{}

func (r JsonString) String() (s string) {
    b, err := json.Marshal(r)
    if err != nil {
        s = ""
        return
    }
    s = string(b)
    return
}

type handler func(rw http.ResponseWriter, req *http.Request)

func jsonResponse(Decored handler) handler {
    return func(rw http.ResponseWriter, req *http.Request) {
        var (
            status string = "success"
            reason string
        )
        defer func() {
            rw.Header().Set("Content-Type", "application/json")
            if r := recover(); r != nil {
                status = "fail"
                reason = r.(string)
            }
            fmt.Fprint(rw, JsonString{"status": status, "reason": reason})
        }()
        Decored(rw, req)
    }
}

func authorize(Decored handler) handler {
    return func(rw http.ResponseWriter, req *http.Request) {
        err := aaa.Authorize(rw, req, true)
        if err != nil {
            fmt.Println(err)
            http.Redirect(rw, req, "/login", http.StatusSeeOther)
            return
        }
        Decored(rw, req)
    }
}

func panicIfErr(e error) {
    if e != nil {
        panic(e.Error())
    }
}

func getDstPath(req *http.Request) string {
    vars := mux.Vars(req)
    path := vars["path"]
    user := context.Get(req, "username").(string)
    return fileroot + "/" + user + "/" + path
}

func addSlash(rw http.ResponseWriter, req *http.Request) {
    http.Redirect(rw, req, req.URL.Path + "/", http.StatusMovedPermanently)
}

func getLogin(rw http.ResponseWriter, req *http.Request) {
    var msg string
    messages := aaa.Messages(rw, req)
    if len(messages) > 0 {
        msg = messages[0]
    }
    err := aaa.Authorize(rw, req, false)
    if err == nil {
        http.Redirect(rw, req, "/api", http.StatusSeeOther)
        return
    }
    if err := tpls.ExecuteTemplate(rw, "login.html", msg); err != nil {
        http.Error(rw, err.Error(), http.StatusInternalServerError)
    }
}

func postLogin(rw http.ResponseWriter, req *http.Request) {
    panicIfErr(req.ParseForm())
    username := req.PostFormValue("username")
    password := req.PostFormValue("password")
    if err := aaa.Login(rw, req, username, password, "/api"); err != nil {
        http.Redirect(rw, req, "/login", http.StatusSeeOther)
    }
}

func postRegister(rw http.ResponseWriter, req *http.Request) {
    panicIfErr(req.ParseForm())
    username := req.PostFormValue("username")
    password := req.PostFormValue("password")
    email := req.PostFormValue("email_address")
    if err := aaa.Register(rw, req, username, password, email); err != nil {
        http.Redirect(rw, req, "/login", http.StatusSeeOther)
    } else {
        panicIfErr(os.MkdirAll(filepath.Join(fileroot, username), 0740))
        postLogin(rw, req)
    }
}

func logout(rw http.ResponseWriter, req *http.Request) {
    if err := aaa.Logout(rw, req); err != nil {
        panicIfErr(err)
        return
    }
    http.Redirect(rw, req, "/login", http.StatusSeeOther)
}

func handleApiRoot(rw http.ResponseWriter, req *http.Request) {
    rw.Header().Set("Content-Type", "application/json")
    var status = "active"
    defer fmt.Fprint(rw, JsonString{
            "status": status,
            "time": time.Now().Format(time.ANSIC),
            "username": context.Get(req, "username")})
    return
}

func handleApiCreate(rw http.ResponseWriter, req *http.Request) {
    var path = getDstPath(req)

    // TODO: Sanitize path, so users can't write to places they shouldn't
    if _, err := os.Stat(path); os.IsNotExist(err) {
        if req.Body == nil {
            panic("No request body provided.")
        }
        body, err := ioutil.ReadAll(req.Body)
        defer req.Body.Close()
        panicIfErr(err);

        panicIfErr(os.MkdirAll(filepath.Dir(path), 0640))
        panicIfErr(ioutil.WriteFile(path, body, 0640))
    } else {
        panic("File exists.")
    }
}

func handleApiMove(rw http.ResponseWriter, req *http.Request) {
    var (
        data map[string]interface{}
        username string = context.Get(req, "username").(string)
    )

    if req.Body == nil {
        panic("No request body.")
    }
    body, err := ioutil.ReadAll(req.Body)
    defer req.Body.Close()

    panicIfErr(json.Unmarshal(body, &data))
    src, oks := data["src"].(string)
    dst, okd := data["dst"].(string)
    if !oks || !okd {
        panic("Invalid json.")
    }
    path := fileroot + "/" + username + "/" + src
    dstpath := fileroot + "/" + username + "/" + dst

    if _, err = os.Stat(path); err != nil {
        panic(err.Error())
    }
    if _, err = os.Stat(dstpath); !os.IsNotExist(err) {
        panic("Overwriting destination file.")
    }
    panicIfErr(os.MkdirAll(filepath.Dir(dstpath), 0640))
    panicIfErr(os.Rename(path, dstpath))
}

func handleApiRemove(rw http.ResponseWriter, req *http.Request) {
    var path = getDstPath(req)
    panicIfErr(os.Remove(path))
}

func handleApiModify(rw http.ResponseWriter, req *http.Request) {
    var path = getDstPath(req)
    // TODO: Sanitize path, so users can't write to places they shouldn't

    if _, err := os.Stat(path); err != nil { panic(err.Error()) }

    body, err := ioutil.ReadAll(req.Body)
    defer req.Body.Close()
    if err != nil { panic(err.Error()) }

    panicIfErr(ioutil.WriteFile(path, body, 0640))
}

func handleApiDownload(rw http.ResponseWriter, req *http.Request) {
    var path = getDstPath(req)

    // TODO: Sanitize path, so users can't write to places they shouldn't
    if _, err := os.Stat(path); os.IsNotExist(err) {
        http.Error(rw, "404 file not found.", http.StatusNotFound)
        return
    } else {
        body, err := ioutil.ReadFile(path)
        if err != nil {
            http.Error(rw, err.Error(), http.StatusInternalServerError)
            return
        }
        _, err = rw.Write(body)
        if err != nil {
            http.Error(rw, err.Error(), http.StatusInternalServerError)
            return
        }
        return
    }
}

func scanDir(root string, recurse bool) (items []JsonString, err error) {
    type finfo struct {
        name string
        t string
        items []JsonString
    }
    var files []os.FileInfo

    files, err = ioutil.ReadDir(root)
    if err != nil {
        return nil, err
    }
    for _, file := range files {
        fstruct := finfo{name: file.Name()}
        if file.IsDir() {
            fstruct.t = "d"
            if recurse {
                fstruct.items, err = scanDir(filepath.Join(root, fstruct.name), recurse)
                if err != nil {
                    return nil, err
                }
            }
        } else {
            fstruct.t = "f"
        }
        if recurse {
            items = append(items, JsonString{"name": fstruct.name, "t": fstruct.t, "items": fstruct.items})
        } else {
            items = append(items, JsonString{"name": fstruct.name, "t": fstruct.t})
        }
    }
    return items, nil
}
func handleWalkDir(rw http.ResponseWriter, req *http.Request, recurse bool) {
    var (
        status string = "success"
        reason string
        items []JsonString
        path = getDstPath(req)
    )
    defer func() {
        if r := recover(); r != nil {
            status = "fail"
            reason = r.(string)
        }
        rw.Header().Set("Content-Type", "application/json")
        fmt.Fprint(rw, JsonString{"status": status, "reason": reason, "items": items})
    }()

    if fi, err := os.Stat(path); err != nil {
        panic(err.Error())
    } else {
        if !fi.Mode().IsDir() { panic("Not a directory.") }
    }

    items, err := scanDir(path, recurse);
    panicIfErr(err)
}

func handleApiList(rw http.ResponseWriter, req *http.Request) { handleWalkDir(rw, req, false) }

func handleApiTree(rw http.ResponseWriter, req *http.Request) { handleWalkDir(rw, req, true) }
