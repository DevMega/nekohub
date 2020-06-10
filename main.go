package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"text/template"

	"github.com/gorilla/context"
	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

type key int

const MyKey key = 0

type User struct {
	ID               uint
	Name             string
	Email            string
	Password         string
	VirtualBrowserID *uint
}

type VirtualBrowser struct {
	ID            uint
	AdminPassword *string
	RoomPassword  *string
	Bind          string
	EPR           string
	ImageID       *string
	ContainerID   *string
	State         int
}

var indexTmpl = template.Must(template.ParseFiles("templates/base.html", "templates/index.html"))
var dashboardTmpl = template.Must(template.ParseFiles("templates/base.html", "templates/dashboard.html"))
var signinTmpl = template.Must(template.ParseFiles("templates/base.html", "templates/signin.html"))
var signupTmpl = template.Must(template.ParseFiles("templates/base.html", "templates/signup.html"))

func (user *User) GetVirtualBrowser() (*VirtualBrowser, error) {
	vb := new(VirtualBrowser)
	row := db.QueryRow("select * from virtual_browsers where id=?", user.VirtualBrowserID)
	err := row.Scan(
		&vb.ID,
		&vb.AdminPassword,
		&vb.RoomPassword,
		&vb.Bind,
		&vb.EPR,
		&vb.ImageID,
		&vb.ContainerID,
		&vb.State,
	)
	if err != nil {
		return nil, err
	}

	return vb, nil
}

func auth(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("auth")
		if err != nil {
			http.Redirect(w, r, "/signin", http.StatusFound)
			return
		}
		values := strings.Split(cookie.Value, "&")
		var user User
		row := db.QueryRow("select id, name, email, password, virtual_browser_id from users where email=?", values[0])
		err = row.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.Password,
			&user.VirtualBrowserID,
		)
		if err != nil {
			log.Fatal(err)
		}
		if user.Password == values[1] {
			context.Set(r, MyKey, user)
			fn(w, r)
			return
		}
		http.Redirect(w, r, "/signin", http.StatusFound)

	}
}

func index(w http.ResponseWriter, r *http.Request) {
	var username string
	cookie, err := r.Cookie("auth")
	if err == nil {
		values := strings.Split(cookie.Value, "&")
		var user User
		row := db.QueryRow("select id, name, email, password from users where email=?", values[0])
		err = row.Scan(
			&user.ID,
			&user.Name,
			&user.Email,
			&user.Password,
		)
		if err != nil {
			http.SetCookie(w, &http.Cookie{
				Name:     "auth",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
			})
		}
		if user.Password == values[1] {
			username = user.Name
		}
	}

	indexTmpl.Execute(w, map[string]interface{}{
		"username": username,
	})
}
func dashboard(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, MyKey).(User)
	virtualBrowser, err := user.GetVirtualBrowser()
	if err != nil {
		log.Fatal(err)
	}
	dashboardTmpl.Execute(w, map[string]interface{}{
		"username": user.Name,
		"vbrowser": virtualBrowser,
	})
}
func signin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	err := r.FormValue("err")
	signinTmpl.Execute(w, map[string]string{
		"username": "",
		"email":    email,
		"err":      err,
	})
}
func signinpost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	var qpassword string
	row := db.QueryRow("select password from users where email=?", email)
	err := row.Scan(&qpassword)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Redirect(w, r, "/signin?email="+email+"&err=201", http.StatusFound)
			return
		}
		log.Fatal(err)
	}
	if password == qpassword {
		http.SetCookie(w, &http.Cookie{
			Name:     "auth",
			Value:    email + "&" + password,
			Path:     "/",
			MaxAge:   60 * 60 * 24 * 30,
			HttpOnly: true,
		})
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/signin?email="+email+"&err=201", http.StatusFound)
}
func signup(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	email := r.FormValue("email")
	err := r.FormValue("err")
	signupTmpl.Execute(w, map[string]string{
		"username": "",
		"name":     name,
		"email":    email,
		"err":      err,
	})
}
func signuppost(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	result, err := tx.Exec("insert into users(name, email, password) values(?,?,?)", name, email, password)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			http.Redirect(w, r, "/signup?name="+name+"&email="+email+"&err=101", http.StatusFound)
			return
		}
		log.Fatal(err)

	}
	userid, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	bind := 8080 + userid
	fepr := 58000 + userid*100
	lepr := 58000 + userid*100 + 99
	result2, err := tx.Exec("insert into virtual_browsers(bind, epr, state) values(?,?,?)",
		strconv.FormatInt(bind, 10),
		strconv.FormatInt(fepr, 10)+"-"+strconv.FormatInt(lepr, 10),
		0,
	)
	if err != nil {
		log.Fatal(err)
	}
	browserid, err := result2.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	_, err = tx.Exec("update users set virtual_browser_id=? where id=?", browserid, userid)
	if err != nil {
		log.Fatal(err)
	}
	tx.Commit()

	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    email + "&" + password,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30,
		HttpOnly: true,
	})
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func signout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "auth",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusFound)
}

func createroom(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, MyKey).(User)
	adminPassword := r.FormValue("adminpassword")
	roomPassword := r.FormValue("roompassword")

	vb, err := user.GetVirtualBrowser()
	if err != nil {
		fmt.Println("1")
		log.Fatal(err)
	}
	fmt.Println(adminPassword)
	fmt.Println(roomPassword)
	fmt.Println(vb.Bind)
	fmt.Println(vb.EPR)

	err = ioutil.WriteFile("dockerfiles/"+user.Email, []byte(fmt.Sprintf(
		"FROM nurdism/neko:firefox\n"+
			"EXPOSE 8080 %s/udp\n"+
			"ENV DISPLAY :99.0\n"+
			"ENV NEKO_PASSWORD %s\n"+
			"ENV NEKO_PASSWORD_ADMIN %s\n"+
			"ENV NEKO_BIND :8080\n"+
			"ENV NEKO_EPR %s", vb.EPR, roomPassword, adminPassword, vb.EPR)), 0600)
	if err != nil {
		log.Fatal(err)
	}
	out, err := exec.Command("docker", "build", "-q", "-f", "dockerfiles/"+user.Email, ".").Output()
	if err != nil {
		log.Fatal(err)
	}
	imageID := fmt.Sprintf("%s", out)[7:71]

	out, err = exec.Command(
		"docker", "run", "-d", "-p", vb.Bind+":8080", "-p", vb.EPR+":"+vb.EPR+"/udp",
		"--restart", "always",
		"--shm-size", "1gb",
		imageID,
	).Output()

	_, err = db.Exec(`update virtual_browsers
		set admin_password=?,
			room_password=?,
			image_id=?,
			container_id=?,
			state=?
		where id=?`,
		adminPassword,
		roomPassword,
		imageID,
		fmt.Sprintf("%s", out)[:64],
		2,
		user.VirtualBrowserID,
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(w, "%s", out)
}

func manageroom(w http.ResponseWriter, r *http.Request) {
	user := context.Get(r, MyKey).(User)
	action := r.FormValue("action")
	vb, err := user.GetVirtualBrowser()
	if err != nil {
		log.Fatal(err)
	}
	switch action {
	case "start":
		out, err := exec.Command("docker", "start", *vb.ContainerID).Output()
		if err != nil {
			log.Fatal(err)
		}
		_, err = db.Exec(`update virtual_browsers
			set state=?
			where id=?`,
			2,
			user.VirtualBrowserID,
		)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(w, "%s", out)
	case "stop":
		out, err := exec.Command("docker", "stop", *vb.ContainerID).Output()
		if err != nil {
			log.Fatal(err)
		}
		_, err = db.Exec(`update virtual_browsers
			set state=?
			where id=?`,
			1,
			user.VirtualBrowserID,
		)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(w, "%s", out)
	case "remove":
		out, err := exec.Command("docker", "rm", *vb.ContainerID).Output()
		if err != nil {
			log.Fatal(err)
		}
		_, err = db.Exec(`update virtual_browsers
			set state=?
			where id=?`,
			0,
			user.VirtualBrowserID,
		)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(w, "%s", out)
	default:
		http.Error(w, "not found action", http.StatusNotFound)
	}
}

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./data.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS 'users' (
			'id'	INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			'name'	TEXT NOT NULL,
			'email'	TEXT NOT NULL UNIQUE,
			'password'	TEXT NOT NULL,
			'virtual_browser_id' INTEGER
		);
		CREATE TABLE IF NOT EXISTS 'virtual_browsers' (
			'id'	INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
			'admin_password'	TEXT,
			'room_password'	TEXT,
			'bind'	TEXT NOT NULL,
			'epr'	TEXT NOT NULL,
			'image_id'	TEXT,
			'container_id'	TEXT,
			'state'	INTEGER NOT NULL
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", index)
	http.HandleFunc("/dashboard", auth(dashboard))
	http.HandleFunc("/signin", signin)
	http.HandleFunc("/signinpost", signinpost)
	http.HandleFunc("/signup", signup)
	http.HandleFunc("/signuppost", signuppost)
	http.HandleFunc("/signout", signout)
	http.HandleFunc("/createroom", auth(createroom))
	http.HandleFunc("/manageroom", auth(manageroom))
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/favicon.ico")
	})
	http.ListenAndServe("0.0.0.0:5000", nil)
}
