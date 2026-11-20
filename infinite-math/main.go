package main

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strconv"
	"strings"

	infimath "github.com/infinage/infimath/src/pkg"
)

type problem struct{ 
	N1, N2, Answer int 
	Iteration int
}

func main() {
	templ, err := template.ParseGlob("assets/html/*html")
	if err != nil {
		fmt.Printf("Template parse fail: %v", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()

	fServer := http.FileServer(http.Dir("./assets"))
	mux.Handle("GET /assets/", http.StripPrefix("/assets", fServer))

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		var problems []problem
		for range 12 {
			n1, n2, ans := infimath.GetAdditionProblem()
			problems = append(problems, problem{N1: n1, N2: n2, Answer: ans})
		}

		err := templ.ExecuteTemplate(w, "Home", problems)
		if err != nil {
			fmt.Println("Template execute fail for '/home': ", err)
		}
	})

	mux.HandleFunc("GET /more", func(w http.ResponseWriter, r *http.Request) {
		iterStr := r.URL.Query().Get("iteration")
		if iterStr == "" {
			http.Error(w, "Missing iteration key", http.StatusBadRequest)
			return
		}

		iters, err := strconv.Atoi(iterStr)
		if err != nil {
			http.Error(w, "Invalid iteration key", http.StatusBadRequest)
			return
		}

		var problemsHTML strings.Builder
		for iter := range iters {
			n1, n2, ans := infimath.GetAdditionProblem()	
			var buf bytes.Buffer
			prob := problem{N1: n1, N2: n2, Answer: ans, Iteration: iters+1}
			err = templ.ExecuteTemplate(&buf, "Problem", prob)
			if err != nil {
				fmt.Printf("Template execute fail for '/more?iterations=%d' [%d]: %v\n", 
					iters, iter, err)
				continue
			}
			problemsHTML.WriteString(buf.String())
		}

		w.Write([]byte(problemsHTML.String()))
	})

	addr := ":8082"
	fmt.Printf("Starting Infinite Math server on %s\n", addr)

	err = http.ListenAndServe(addr, mux)
	if err != nil {
		fmt.Printf("Serve fail: %v", err)
		os.Exit(1)
	}
}
