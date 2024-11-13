package main

import (
	"flag"
	"todo-list/database"
	"todo-list/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	// 定义命令行参数
	dbPath := flag.String("db", "./todo.db", "Path to the SQLite database file")
	flag.Parse()

	db := database.InitDB(*dbPath)

	handlers.SetDB(db)

	r := gin.Default()

	r.GET("/userinfo", handlers.GetUserInfo)

	r.GET("/todos", handlers.GetTodos)
	r.POST("/todos", handlers.CreateTodo)
	r.PUT("/todos/:id", handlers.UpdateTodo)
	r.DELETE("/todos/:id", handlers.DeleteTodo)

	r.Run(":3000")
}
