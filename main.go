package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/thedevsaddam/renderer"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var rnd *renderer.Render // renderer instance
var db *mgo.Database     // mongodb database instance

// constants used in the application
const (
	hostName       string = "localhost:27017"
	port           string = ":9000"
	dbName         string = "demo_todo"
	collectionName string = "todo"
)

type (

	// TodoModel struct is used to store the todo data in mongodb
	todoModel struct {
		ID        bson.ObjectId `bson:"_id,omitempty"`
		Title     string        `bson:"title"`
		Completed bool          `bson:"completed"`
		CreatedAt time.Time     `bson:"created_at"`
	}

	// Todo struct is used to render the todo data
	todo struct {
		ID        string    `json:"id"`
		Title     string    `json:"title"`
		Completed bool      `json:"completed"`
		CreatedAt time.Time `json:"created_at"`
	}
)

func init() {
	rnd = renderer.New()              // initialize the renderer
	sess, err := mgo.Dial(hostName)   // connect to mongodb
	checkErr(err)                     // check for error
	sess.SetMode(mgo.Monotonic, true) // set the session mode to monotonic
	db = sess.DB(dbName)              // get the database
}

func homeHandler(w http.ResponseWriter, r *http.Request) { // home handler
	err := rnd.Template(w, http.StatusOK, []string{"static/home.tpl"}, nil) // render the home template
	checkErr(err)                                                          // check for error
}

func fetchTodos(w http.ResponseWriter, r *http.Request) { // fetch todos handler
	todos := []todoModel{} // initialize the todos slice

	if err := db.C(collectionName).Find(bson.M{}).All(&todos); err != nil { // fetch all the todos from mongodb
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Error fetching todos",
			"error":   err,

		})
		return 
}
todoList := []todo{} // initialize the todo list

for _, t := range todos { // loop through the todos
	todoList = append(todoList, todo{ // append the todo to the todo list
		ID:        t.ID.Hex(), // convert the object id to hex
		Title:     t.Title,    // set the title
		Completed: t.Completed, // set the completed status
		CreatedAt: t.CreatedAt, // set the created at
	})
}

rnd.JSON(w, http.StatusOK, renderer.M{
	"data": todoList, // set the todo list
})
}

func createTodo(w http.ResponseWriter, r *http.Request) { // create todo handler
	var t todo

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil { // decode the request body to todo struct
		rnd.JSON(w, http.StatusProcessing, err)
		return
}

if t.Title == "" { // check if the title is empty
	rnd.JSON(w, http.StatusBadRequest, renderer.M{
		"message": "Title is required",
	})
	return
}

tm := todoModel{ // create a todo model
	ID:        bson.NewObjectId(), // generate a new object id
	Title:     t.Title,            // set the title
	Completed: false,              // set the completed status
	CreatedAt: time.Now(),         // set the created at
}

if err := db.C(collectionName).Insert(&tm); err != nil { // insert the todo model to mongodb
	rnd.JSON(w, http.StatusProcessing, renderer.M{
		"message": "Error creating todo",
		"error":   err,
	})	
	return
}

rnd.JSON(w, http.StatusCreated, renderer.M{// return the created todo model
	"message": "Todo created successfully",
	"todo_id": tm.ID.Hex()
})
}

func deleteTodo(w http.ResponseWriter, r *http.Request) { // delete todo handler
	id := strings.TrimSpace(chi.URLParam(r, "id")) // get the todo id from the url

	if !bson.IsObjectIdHex(id) { // check if the todo id is valid
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid todo id",
		})
		return
	}


	if err := db.C(collectionName).RemoveId(bson.ObjectIdHex(todoID)); err != nil { // delete the todo from mongodb
		rnd.JSON(w, http.StatusProcessing, renderer.M{
			"message": "Error deleting todo",
			"error":   err,
		})
		return
	}

	rnd.JSON(w, http.StatusOK, renderer.M{
		"message": "Todo deleted successfully",
	})
}

func updateTodo(w http.ResponseWriter, r *http.Request) { // update todo handler
	id := strings.TrimSpace(chi.URLParam(r, "id")) // get the todo id from the url

	if !bson.IsObjectIdHex(id) { // check if the todo id is valid
		rnd.JSON(w, http.StatusBadRequest, renderer.M{
			"message": "Invalid todo id",
		})
		return
}

var t todo

if err := json.NewDecoder(r.Body).Decode(&t); err != nil { // decode the request body to todo struct
	rnd.JSON(w, http.StatusProcessing, err)
	return
}

if t.Title == "" { // check if the title is empty
	rnd.JSON(w, http.StatusBadRequest, renderer.M{
		"message": "Title is required",
	})
	return
}

if err := db.C(collectionName).
Update(
	bson.M{"_id": bson.ObjectIdHex(id)}, // query
	bson.M{"title": t.Title, "completed": t.Completed}, // update
); err != nil { // update the todo in mongodb
	rnd.JSON(w, http.StatusProcessing, renderer.M{
		"message": "Error updating todo",
		"error":   err,
	})
	return
}}

func main() {
	stopChan := make(chan os.Signal)      // channel to receive os interrupt signal
	signal.Notify(stopChan, os.Interrupt) // notify the channel when os interrupt signal is received
	r := chi.NewRouter()                  // initialize the router
	r.Use(middleware.Logger)              // use the logger middleware
	r.Get("/", homeHandler)               // handle the home route
	r.Mount("/todo", todoRouters())       // mount the todo router

	// start the server
	srv := &http.Server{
		Addr:         port,              // set the port
		Handler:      r,                 // set the default handler
		ReadTimeout:  60 * time.Second,  // set the read timeout
		WriteTimeout: 60 * time.Second,  // set the write timeout
		IdleTimeout:  120 * time.Second, // set the idle timeout
	}

	//idle is a channel that will receive a value when the server is idle

	//start the server in a goroutine
	go func() {
		log.Println("Listening on port", port)       // print the listening port
		if err := srv.ListenAndServe(); err != nil { // start the server
			log.Printf("listen: %s\n", err) // print the error
		}
	}()

	<-stopChan                                                              // wait for the os interrupt signal
	log.Println("Shutting down the server...")                              // print the message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // create a context with timeout
	srv.Shutdown(ctx)                                                       // shutdown the server
	defer cancel(
		log.Println("Server gracefully stopped") 				
	)
	}                                                       

func todoHandlers() http.Handler { // todo handlers
	rg := chi.NewRouter()         // initialize the router
	rg.Group(func(r chi.Router) { // group the routes
		r.Get("/", fetchTodos)        // handle the fetch todos route
		r.Post("/", createTodo)       // handle the create todo route
		r.Put("/{id}", updateTodo)    // handle the update todo route
		r.Delete("/{id}", deleteTodo) // handle the delete todo route
	})
	return rg // return the router
}

func checkErr(err error) { // check for error
	if err != nil {       // check if error is not nil then print the error and exit
		log.Fatal(err)   // print the error
	}
}
