package cmd

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	nlog "github.com/nuveo/log"
	//"github.com/prest/adapters/postgres"
	mysql "github.com/prest/adapter-mysql"
	"github.com/prest/config"
	"github.com/prest/config/router"
	"github.com/prest/controllers"
	"github.com/prest/middlewares"
	"github.com/spf13/cobra"
	"github.com/urfave/negroni"
	// postgres driver for migrate
	_ "gopkg.in/mattes/migrate.v1/driver/postgres"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "prest",
	Short: "Serve a RESTful API from any PostgreSQL database",
	Long:  `Serve a RESTful API from any PostgreSQL database, start HTTP server`,
	Run: func(cmd *cobra.Command, args []string) {
		if config.PrestConf.Adapter == nil {
			nlog.Warningln("adapter is not set. Using the default (postgres)")
			mysql.Load()
		}
		if config.PrestConf.SocketPath !="" {
			go func (){
				startSocketServer()
			}()
		}
		startServer()
		
	},
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	migrateCmd.AddCommand(createCmd)
	migrateCmd.AddCommand(downCmd)
	migrateCmd.AddCommand(gotoCmd)
	migrateCmd.AddCommand(mversionCmd)
	migrateCmd.AddCommand(nextCmd)
	migrateCmd.AddCommand(redoCmd)
	migrateCmd.AddCommand(upCmd)
	migrateCmd.AddCommand(resetCmd)
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(migrateCmd)
	migrateCmd.PersistentFlags().StringVar(&urlConn, "url", driverURL(), "Database driver url")
	migrateCmd.PersistentFlags().StringVar(&path, "path", config.PrestConf.MigrationsPath, "Migrations directory")

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}

// MakeHandler reagister all routes
func MakeHandler() http.Handler {
	n := middlewares.GetApp()
	r := router.Get()
	r.HandleFunc("/databases", controllers.GetDatabases).Methods("GET")
	r.HandleFunc("/schemas", controllers.GetSchemas).Methods("GET")
	r.HandleFunc("/tables", controllers.GetTables).Methods("GET")
	r.HandleFunc("/_QUERIES/{queriesLocation}/{script}", controllers.ExecuteFromScripts)
	r.HandleFunc("/{database}/{schema}", controllers.GetTablesByDatabaseAndSchema).Methods("GET")
	crudRoutes := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.SelectFromTables).Methods("GET")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.InsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/batch/{database}/{schema}/{table}", controllers.BatchInsertInTables).Methods("POST")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.DeleteFromTable).Methods("DELETE")
	crudRoutes.HandleFunc("/{database}/{schema}/{table}", controllers.UpdateTable).Methods("PUT", "PATCH")
	r.PathPrefix("/").Handler(negroni.New(
		middlewares.AccessControl(),
		negroni.Wrap(crudRoutes),
	))
	n.UseHandler(r)
	return n
}

func startServer() {

	mux := http.NewServeMux()
	mux.Handle(config.PrestConf.ContextPath, MakeHandler())
	l := log.New(os.Stdout, "[prest] ", 0)

	if !config.PrestConf.AccessConf.Restrict {
		nlog.Warningln("You are running pREST in public mode.")
	}

	if config.PrestConf.Debug {
		nlog.DebugMode = config.PrestConf.Debug
		nlog.Warningln("You are running pREST in debug mode.")
	}
	addr := fmt.Sprintf("%s:%d", config.PrestConf.HTTPHost, config.PrestConf.HTTPPort)
	l.Printf("listening on %s and serving on %s", addr, config.PrestConf.ContextPath)
	if config.PrestConf.HTTPSMode {
		l.Fatal(http.ListenAndServeTLS(addr, config.PrestConf.HTTPSCert, config.PrestConf.HTTPSKey, mux))
	}
	l.Fatal(http.ListenAndServe(addr, mux))
}

// socket 服务
func startSocketServer() {
	 mux := http.NewServeMux()
	 mux.Handle(config.PrestConf.ContextPath, MakeHandler())
	l := log.New(os.Stdout, "[prest] ", 0)

	if !config.PrestConf.AccessConf.Restrict {
		nlog.Warningln("You are running pREST in public mode.")
	}

	if config.PrestConf.Debug {
		nlog.DebugMode = config.PrestConf.Debug
		nlog.Warningln("You are running pREST in debug mode.")
	}
	l.Printf("listening on %s and serving on %s", config.PrestConf.SocketPath,config.PrestConf.ContextPath)
	if err := os.RemoveAll(config.PrestConf.SocketPath); err != nil {
        l.Fatal(err)
    }
	unixListener,err := net.Listen("unix",config.PrestConf.SocketPath)
	if err!=nil{
		l.Fatal(err)
	}
	defer unixListener.Close()
	l.Fatal(http.Serve(unixListener,mux))

}
