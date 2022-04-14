package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"sync"
)

// User represents a real world user of a particular system.
type User struct {
	// ID is the API automatically generated user identification number.
	//
	// This ID is generated when a POST user request is made.
	ID int `json:"id"`

	// Username is the username of the user of the API.
	Username string `json:"username"`

	// Password is the password of the user of the API.
	Password string `json:"password"`

	// Email is the email of the user of the API.
	Email string `json:"email"`
}

// DataStore represents an in-memory data store for the API.
type DataStore struct {
	// store represents the in-memory data store.
	store map[int]*User

	*sync.RWMutex
}

// UserHandler is the HTTP handler for the http.Handler interface.
type UserHandler struct {
	// users is the data store for the users of the API.
	users DataStore
}

// currUserId store the id of the last user created.
var currUserId = -1

// nextUserId generate the next id for the user that is being created.
func nextUserId() int {
	currUserId += 1
	return currUserId
}

// ServeHTTP is the HTTP handler implementation of the UserHandler.
func (h *UserHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	// set API to be JSON based
	rw.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		h.get(rw, r)

	case http.MethodPost:
		h.create(rw, r)

	case http.MethodPut:
		h.update(rw, r)

	case http.MethodDelete:
		h.delete(rw, r)

	default:
		errMsg := "HTTP verb not implemented"
		http.Error(rw, errMsg, http.StatusNotImplemented)
		log.Println("[INFO]", r.Method, errMsg)
	}
}

// get retrieve all or specific user information to the client.
func (h *UserHandler) get(rw http.ResponseWriter, r *http.Request) {
	// retrieve all users
	listUsersRe := regexp.MustCompile(`^/users[/]*$`)

	if listUsersRe.MatchString(r.URL.Path) {
		log.Println("[INFO] received a GET all users request")

		// secure concurrent access on the data store
		h.users.RLock()

		users := make([]*User, 0, len(h.users.store))
		for _, v := range h.users.store {
			users = append(users, v)
		}

		h.users.RUnlock()

		// try to encode users to JSON and return it to client
		if err := json.NewEncoder(rw).Encode(users); err != nil {
			errMsg := "an error occured while trying to get users"
			http.Error(rw, errMsg, http.StatusInternalServerError)
			log.Println("[ERROR] "+errMsg, err.Error())
		}

		return
	}

	// get single user
	getUserRe := regexp.MustCompile(`^/users/(\d+)$`)

	if getUserRe.MatchString(r.URL.Path) {
		log.Println("[INFO] received a GET user request")

		// try to parse user {id} from the request URL
		matches := getUserRe.FindStringSubmatch(r.URL.Path)
		if len(matches) < 2 {
			errMsg := "invalid user id"
			http.Error(rw, errMsg, http.StatusBadRequest)
			log.Println("[INFO] " + errMsg)
			return
		}
		id, _ := strconv.Atoi(matches[1])

		// secure concurrent access on the data store
		h.users.RLock()
		defer h.users.RUnlock()

		// try to get user on the data store
		user, ok := h.users.store[id]
		if !ok {
			errMsg := fmt.Sprintf("user %d not found", id)
			http.Error(rw, errMsg, http.StatusNotFound)
			log.Println("[INFO] " + errMsg)
			return
		}

		// try to encode user to JSON and return it to client
		if err := user.toJSON(rw); err != nil {
			errMsg := "an error occured while trying to get user"
			http.Error(rw, errMsg, http.StatusInternalServerError)
			log.Println("[ERROR] "+errMsg, err.Error())
		}
	}
}

// create receive a user payload, create and add a new user to the
// in-memory data store.
func (h *UserHandler) create(rw http.ResponseWriter, r *http.Request) {
	log.Println("[INFO] received a POST user request")

	// try to parse User from request payload
	user := &User{}
	if err := user.fromJSON(r.Body); err != nil {
		errMsg := "an error occured while parsing user payload"
		http.Error(rw, errMsg, http.StatusBadRequest)
		log.Println("[ERROR]", errMsg, err.Error())
		return
	}

	// secure concurrent access on the data store
	h.users.Lock()
	defer h.users.Unlock()

	// add user on data store
	user.ID = nextUserId()
	h.users.store[user.ID] = user

	// try to return created user
	if err := user.toJSON(rw); err != nil {
		errMsg := "an error occured while trying to retrieve created user"
		http.Error(rw, errMsg, http.StatusInternalServerError)
		log.Println("[ERROR]", errMsg, err.Error())
	}
}

// update receive a user payload information and update the
// corresponding user with the {id} parameter of the request.
func (h *UserHandler) update(rw http.ResponseWriter, r *http.Request) {
	log.Println("[INFO] received a PUT user request")

	// parse user {id} from the request URL
	updateUserRe := regexp.MustCompile(`^/users/(\d+)$`)

	matches := updateUserRe.FindStringSubmatch(r.URL.Path)
	if len(matches) < 2 {
		errMsg := "invalid user id"
		http.Error(rw, errMsg, http.StatusBadRequest)
		log.Println("[INFO]", errMsg)
		return
	}
	id, _ := strconv.Atoi(matches[1])

	// parse user data from request payload
	user := &User{}
	if err := user.fromJSON(r.Body); err != nil {
		errMsg := "invalid payload data"
		http.Error(rw, errMsg, http.StatusBadRequest)
		log.Println("[ERROR]", errMsg, err.Error())
		return
	}

	// secure concurrent access to data store
	h.users.Lock()
	defer h.users.Unlock()

	// check if user exist on data store
	if _, ok := h.users.store[id]; !ok {
		errMsg := fmt.Sprintf("user %d not found", id)
		http.Error(rw, errMsg, http.StatusNotFound)
		log.Println("[INFO]", errMsg)
		return
	}

	// update user
	user.ID = id
	h.users.store[id] = user

	// try to return updated user
	if err := user.toJSON(rw); err != nil {
		errMsg := "an error occured while trying to get user"
		http.Error(rw, errMsg, http.StatusInternalServerError)
		log.Println("[ERROR]", errMsg, err.Error())
	}
}

// delete removes a user from the datastore
func (h *UserHandler) delete(rw http.ResponseWriter, r *http.Request) {
	log.Println("[INFO] received a DELETE user request")

	// parse user {id} from the request URL
	deleteUserRe := regexp.MustCompile(`^/users/(\d+)$`)

	matches := deleteUserRe.FindStringSubmatch(r.URL.Path)
	if len(matches) < 2 {
		errMsg := "invalid user id"
		http.Error(rw, errMsg, http.StatusBadRequest)
		log.Println("[INFO]", errMsg)
		return
	}
	id, _ := strconv.Atoi(matches[1])

	// secure concurrent access to data store
	h.users.Lock()
	defer h.users.Unlock()

	// check if user exist on the data store
	if _, ok := h.users.store[id]; !ok {
		errMsg := fmt.Sprintf("user %d not found", id)
		http.Error(rw, errMsg, http.StatusNotFound)
		log.Println("[INFO]", errMsg)
		return
	}

	// delete user from data store
	user := &User{}
	*user = *h.users.store[id]
	delete(h.users.store, id)

	// try to return deleted user
	if err := user.toJSON(rw); err != nil {
		errMsg := "an error occured while trying to get the deleted user:"
		http.Error(rw, errMsg, http.StatusInternalServerError)
		log.Println("[ERROR]", errMsg, err.Error())
	}
}

// toJSON tries to encodes current user information to JSON format onto the
// io.Writer object.
func (u *User) toJSON(w io.Writer) error {
	return json.NewEncoder(w).Encode(u)
}

// fromJSON tries to decode the payload into the current user from the
// io.Reader object.
func (u *User) fromJSON(r io.Reader) error {
	return json.NewDecoder(r).Decode(u)
}

func main() {
	// create the handler for the HTTP requests
	userHandler := &UserHandler{
		users: DataStore{
			store:   map[int]*User{},
			RWMutex: &sync.RWMutex{},
		},
	}

	mux := http.NewServeMux()
	mux.Handle("/users", userHandler)
	mux.Handle("/users/", userHandler)

	// Initialize and run server
	const PORT = "8080"
	log.Printf("server starting at http://localhost:%s/\n", PORT)

	log.Fatalln(http.ListenAndServe(fmt.Sprintf(":%s", PORT), mux))
}
