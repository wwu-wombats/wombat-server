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
)

type SiteData struct {
    Root string
}

var (
    apiroot string = "/api"
    ctx SiteData = SiteData{Root: "localhost"}
    tpls = template.Must(template.ParseFiles("tpls/login.html"))
    fileroot string = "files"
    username string = "apexskier"
)

func main() {
    fmt.Println("Starting server on port 8080.")
    r := mux.NewRouter()
    r.HandleFunc("/login", getLogin).Methods("GET")
    r.HandleFunc("/login", postLogin).Methods("POST")

    // Static files
    http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir("static"))))

    // API
    r.HandleFunc(apiroot + "/", handleApiRoot).Methods("GET")
    r.HandleFunc(apiroot + "/create/{path:.*}", handleApiCreate).Methods("POST")
    r.HandleFunc(apiroot + "/move", handleApiMove).Methods("POST")
    r.HandleFunc(apiroot + "/delete/{path:.*}", handleApiRemove).Methods("POST")
    r.HandleFunc(apiroot + "/modify/{path:.*}", handleApiModify).Methods("POST")
    r.HandleFunc(apiroot + "/download/{path:.*}", handleApiDownload).Methods("GET")
    r.HandleFunc(apiroot + "/list/{path:.*}", handleApiList).Methods("GET")
    r.HandleFunc(apiroot + "/tree/{path:.*}", handleApiTree).Methods("GET")

    http.Handle("/", r)
    http.ListenAndServe(":8080", nil)
}

type JsonResponse map[string]interface{}

func (r JsonResponse) String() (s string) {
    b, err := json.Marshal(r)
    if err != nil {
        s = ""
        return
    }
    s = string(b)
    return
}

func getLogin(rw http.ResponseWriter, req *http.Request) {
    err := tpls.ExecuteTemplate(rw, "login.html", ctx)
    if err != nil {
        http.Error(rw, err.Error(), http.StatusInternalServerError)
    }
}

func postLogin(rw http.ResponseWriter, req *http.Request) {
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": "success", "time": time.Now().Format(time.ANSIC)})
    return
}

func handleApiRoot(rw http.ResponseWriter, req *http.Request) {
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": "active", "time": time.Now().Format(time.ANSIC)})
    return
}

func handleApiCreate(rw http.ResponseWriter, req *http.Request) {
    var (
        vars = mux.Vars(req)
        status string = "success"
        reason string = ""
        path = fileroot + "/" + username + "/" + vars["path"]
    )

    // TODO: Sanitize path, so users can't write to places they shouldn't
    if _, err := os.Stat(path); os.IsNotExist(err) {
        if req.Body == nil {
            return
        }
        body, err := ioutil.ReadAll(req.Body)
        req.Body.Close()

        err = os.MkdirAll(filepath.Dir(path), 0740)
        if err != nil {
            reason = err.Error()
            status = "fail"
        } else {
            err = ioutil.WriteFile(path, body, 0740)
            if err != nil {
                reason = err.Error()
                status = "fail"
            }
        }
    } else {
        status = "fail"
        reason = "File exists."
    }
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": status, "reason": reason})
    return
}

func handleApiMove(rw http.ResponseWriter, req *http.Request) {
    var (
        data map[string]interface{}
        reason string = ""
        status string = "success"
    )
    if req.Body == nil {
        status = "fail"
        reason = "No request body."
    } else {
        if req.Body == nil {
            return
        }
        body, err := ioutil.ReadAll(req.Body)
        req.Body.Close()

        err = json.Unmarshal(body, &data)
        if err != nil {
            reason = err.Error()
            status = "fail"
        } else {
            if src, oks := data["src"].(string); oks {
                if dst, okd := data["dst"].(string); okd {
                    path := fileroot + "/" + username + "/" + src
                    dstpath := fileroot + "/" + username + "/" + dst
                    if _, err = os.Stat(path); err == nil {
                        err = os.MkdirAll(filepath.Dir(dstpath), 0740)
                        if err != nil {
                            reason = err.Error()
                            status = "fail"
                        } else {
                            err = os.Rename(path, dstpath)
                            if err != nil {
                                reason = err.Error()
                                status = "fail"
                            }
                        }
                    } else {
                        reason = "File doesn't exist."
                        status = "fail"
                    }
                } else {
                    reason = "Invalid json."
                    status = "fail"
                }
            } else {
                reason = "Invalid json."
                status = "fail"
            }
        }
    }
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": status, "reason": reason})
    return
}

func handleApiRemove(rw http.ResponseWriter, req *http.Request) {
    var (
        vars = mux.Vars(req)
        status string = "success"
        reason string = ""
        path = fileroot + "/" + username + "/" + vars["path"]
    )
    err := os.Remove(path)
    if err != nil {
        status = "fail"
        reason = err.Error()
    }
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": status, "reason": reason})
    return
}

func handleApiModify(rw http.ResponseWriter, req *http.Request) {
    var (
        vars = mux.Vars(req)
        status string = "success"
        reason string = ""
        path = fileroot + "/" + username + "/" + vars["path"]
    )

    // TODO: Sanitize path, so users can't write to places they shouldn't
    if _, err := os.Stat(path); err == nil {
        if req.Body == nil {
            return
        }
        body, err := ioutil.ReadAll(req.Body)
        req.Body.Close()

        err = ioutil.WriteFile(path, body, 0740)
        if err != nil {
            reason = err.Error()
            status = "fail"
        }
    } else {
        status = "fail"
        reason = err.Error()
    }
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": status, "reason": reason})
    return
}

func handleApiDownload(rw http.ResponseWriter, req *http.Request) {
    var (
        vars = mux.Vars(req)
        path = fileroot + "/" + username + "/" + vars["path"]
    )

    // TODO: Sanitize path, so users can't write to places they shouldn't
    if _, err := os.Stat(path); os.IsNotExist(err) {
        rw.WriteHeader(http.StatusNotFound)
        return
    } else {
        body, err := ioutil.ReadFile(path)
        if err != nil {
            rw.WriteHeader(http.StatusInternalServerError)
            return
        }
        _, err = rw.Write(body)
        if err != nil {
            rw.WriteHeader(http.StatusInternalServerError)
            return
        }
        return
    }
}

func scanDir(root string, recurse bool) (items []JsonResponse, err error) {
    type finfo struct {
        name string
        t string
        items []JsonResponse
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
            items = append(items, JsonResponse{"name": fstruct.name, "t": fstruct.t, "items": fstruct.items})
        } else {
            items = append(items, JsonResponse{"name": fstruct.name, "t": fstruct.t})
        }
    }
    return items, nil
}

func handleApiList(rw http.ResponseWriter, req *http.Request) {
    var (
        vars = mux.Vars(req)
        status string = "success"
        reason string = ""
        items []JsonResponse
        path = fileroot + "/" + username + "/" + vars["path"]
    )

    if fi, err := os.Stat(path); err == nil {
        if fi.Mode().IsDir() {
            items, err = scanDir(path, false)
            if err != nil {
                status = "fail"
                reason = err.Error()
            }
        } else {
            status = "fail"
            reason = "Not a directory."
        }
    } else {
        status = "fail"
        reason = err.Error()
    }
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": status, "reason": reason, "items": items})
    return
}

func handleApiTree(rw http.ResponseWriter, req *http.Request) {
    var (
        vars = mux.Vars(req)
        status string = "success"
        reason string = ""
        items []JsonResponse
        path = fileroot + "/" + username + "/" + vars["path"]
    )

    if fi, err := os.Stat(path); err == nil {
        if fi.Mode().IsDir() {
            items, err = scanDir(path, true)
            if err != nil {
                status = "fail"
                reason = err.Error()
            }
        } else {
            status = "fail"
            reason = "Not a directory."
        }
    } else {
        status = "fail"
        reason = err.Error()
    }
    rw.Header().Set("Content-Type", "application/json")
    fmt.Fprint(rw, JsonResponse{"status": status, "reason": reason, "items": items})
    return
}
