package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Peer struct {
	Addr string `json:"Addr"`
}

type Asset struct {
	Key   string `json:"Key"`
	Value int    `json:"Value"`
}

type MPCRequest struct {
	Expr   string `json:"Expr"`
	Budget int    `json:"Budget"`
}

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello, you've requested: %s. which is a 404 because it is not implemented yet.\n", r.URL.Path)
}

func peerHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// The request is a POST request
		var peer Peer
		err := json.NewDecoder(r.Body).Decode(&peer)
		if err != nil {
			http.Error(w, "Invalid post request", http.StatusBadRequest)
			return
		}

		fmt.Println("Adding peer:", peer.Addr)
		// TODO: add peer

		// Set the content type to JSON
		w.Header().Set("Content-Type", "application/json")

		// Marshal the user object into a JSON object
		json, _ := json.Marshal(peer)

		// Write the JSON object to the response
		w.Write(json)
	}
}

func assetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// The request is a GET request
		fmt.Println("showing assets:")

		// Set the content type to JSON
		w.Header().Set("Content-Type", "application/json")

		assets := map[string]map[string]int{}
		assets["peerA"] = map[string]int{
			"a1": 3,
			"a2": 4,
		}
		assets["peerB"] = map[string]int{
			"b": 5,
		}
		// TODO: get real assets

		json, _ := json.Marshal(assets)
		w.Write(json)

	} else if r.Method == http.MethodPost {
		// The request is a POST request
		var asset Asset
		err := json.NewDecoder(r.Body).Decode(&asset)
		if err != nil {
			http.Error(w, "Invalid post request", http.StatusBadRequest)
			return
		}

		fmt.Printf("Adding assets key: %s, value: %d\n", asset.Key, asset.Value)
		// TODO: add assets

		// Set the content type to JSON
		w.Header().Set("Content-Type", "application/json")
		json, _ := json.Marshal(asset)
		w.Write(json)
	}
}

func balanceHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// The request is a GET request
		fmt.Println("showing balance:")

		// Set the content type to JSON
		w.Header().Set("Content-Type", "application/json")

		balance := map[string]int{}
		balance["peerA"] = 10
		balance["peerB"] = 20
		balance["peerC"] = 30
		// TODO: get real balance

		json, _ := json.Marshal(balance)
		w.Write(json)

	}
}

func blockchainHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		// The request is a GET request
		fmt.Println("showing blockchain:")

		// Set the content type to JSON
		w.Header().Set("Content-Type", "application/json")

		blocks := map[string]string{}
		blocks["block1"] = "hahahe"
		blocks["block2"] = "hehehe"

		// TODO: get real block

		json, _ := json.Marshal(blocks)
		w.Write(json)

	}
}

func mpcHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		// The request is a POST request
		var mpcRequest MPCRequest
		err := json.NewDecoder(r.Body).Decode(&mpcRequest)
		if err != nil {
			http.Error(w, "Invalid post request", http.StatusBadRequest)
			return
		}

		fmt.Printf("MPC Calculate expr: %s, budget: %d\n", mpcRequest.Expr, mpcRequest.Budget)
		// TODO: calling real MPC

		// Set the content type to JSON
		w.Header().Set("Content-Type", "application/json")
		json, _ := json.Marshal(mpcRequest)
		w.Write(json)
	}
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/peer", peerHandler)
	http.HandleFunc("/asset", assetHandler)
	http.HandleFunc("/balance", balanceHandler)
	http.HandleFunc("/blockchain", blockchainHandler)
	http.HandleFunc("/mpcCalculate", mpcHandler)

	http.ListenAndServe(":8080", nil)

	// // add peer
	// curl -X POST -H "Content-Type: application/json" -d '{"Addr":"127.0.0.1:2"}' http://127.0.0.1:8080/peer

	// // get/add assets
	// curl http://127.0.0.1:8080/asset
	// curl -X POST -H "Content-Type: application/json" -d '{"key":"a", "value":3}' http://127.0.0.1:8080/asset

	// // get balance
	// curl http://127.0.0.1:8080/balance

	// // get blockchain
	// curl http://127.0.0.1:8080/blockchain

	// // post mpcCalculate
	// curl -X POST -H "Content-Type: application/json" -d '{"Expr":"a+b", "Budget":10}' http://127.0.0.1:8080/mpcCalculate

}
