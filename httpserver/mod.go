package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
	z "go.dedis.ch/cs438/internal/testing"
)

type PeerJson struct {
	Addr string `json:"Addr"`
}

type AssetJson struct {
	Key   string  `json:"Key"`
	Value int     `json:"Value"`
	Price float64 `json:"Price"`
}

type MPCRequestJson struct {
	Expr   string `json:"Expr"`
	Budget int    `json:"Budget"`
}

type MPCReplyJson struct {
	Expr    string  `json:"Expr"`
	Balance float64 `json:"Balance"`
	Result  int     `json:"Result"`
}

func handler(n *z.TestNode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Error().Msgf("got 404 since the url is unrecognized!!.")
		fmt.Fprintf(w, "Hello, you've requested: %s. which is a 404 because it is not implemented yet.\n", r.URL.Path)
	}
}

func peerHandler(n *z.TestNode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodPost {
			// The request is a POST request
			var peer PeerJson
			err := json.NewDecoder(r.Body).Decode(&peer)
			if err != nil {
				http.Error(w, "Invalid post request", http.StatusBadRequest)
				return
			}

			log.Info().Msgf("HTTP request Adding peer:", peer.Addr)
			n.AddPeer(peer.Addr)

			// Set the content type to JSON
			w.Header().Set("Content-Type", "application/json")

			// Marshal the user object into a JSON object
			json, _ := json.Marshal(peer)

			// Write the JSON object to the response
			w.Write(json)
		}
	}
}

func assetHandler(n *z.TestNode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodGet {
			// The request is a GET request

			// Set the content type to JSON
			w.Header().Set("Content-Type", "application/json")

			// assets := map[string]map[string]float64{}
			json, err := json.Marshal(n.GetAllPeerAssetPrices())
			if err != nil {
				http.Error(w, "Failed to get assets", http.StatusBadRequest)
				return
			}
			w.Write(json)

		} else if r.Method == http.MethodPost {
			// The request is a POST request
			var asset AssetJson
			err := json.NewDecoder(r.Body).Decode(&asset)
			if err != nil {
				http.Error(w, "Invalid post request", http.StatusBadRequest)
				return
			}

			log.Info().Msgf("HTTP adding assets key: %s, value: %d, price: %f", asset.Key, asset.Value, asset.Price)
			err = n.SetValueDBAsset(asset.Key, asset.Value, asset.Price)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// Set the content type to JSON
			w.Header().Set("Content-Type", "application/json")
			json, _ := json.Marshal(asset)
			w.Write(json)
		}
	}
}

func balanceHandler(n *z.TestNode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodGet {
			// The request is a GET request
			// fmt.Println("showing balance:")

			// Set the content type to JSON
			w.Header().Set("Content-Type", "application/json")

			balance := map[string]float64{}
			addr, _ := n.BCGetAddress()
			balance[addr.Hex] = n.BCGetBalance()

			json, _ := json.Marshal(balance)
			w.Write(json)

		}
	}
}

func blockchainHandler(n *z.TestNode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
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
}

func mpcHandler(n *z.TestNode) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodPost {
			// The request is a POST request
			var mpcRequest MPCRequestJson
			err := json.NewDecoder(r.Body).Decode(&mpcRequest)
			if err != nil {
				http.Error(w, "Invalid post request", http.StatusBadRequest)
				return
			}

			log.Info().Msgf("Http MPC Calculate expr: %s, budget: %d", mpcRequest.Expr, mpcRequest.Budget)
			ans, err := n.Calculate(mpcRequest.Expr, float64(mpcRequest.Budget))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			log.Info().Msgf("Http MPC calculate done, expr: %s, result: %d", mpcRequest.Expr, ans)

			mpcResult := MPCReplyJson{}
			mpcResult.Balance = n.BCGetBalance()
			mpcResult.Result = ans
			mpcResult.Expr = mpcRequest.Expr
			// Set the content type to JSON
			w.Header().Set("Content-Type", "application/json")
			json, _ := json.Marshal(mpcResult)
			w.Write(json)
		}
	}
}

func MainHttp(n *z.TestNode, port string) {
	http.HandleFunc("/", handler(n))
	http.HandleFunc("/peer", peerHandler(n))
	http.HandleFunc("/asset", assetHandler(n))
	http.HandleFunc("/balance", balanceHandler(n))
	http.HandleFunc("/blockchain", blockchainHandler(n))
	http.HandleFunc("/mpcCalculate", mpcHandler(n))

	http.ListenAndServe(port, nil)

	// go run main.go cli
	// go run main.go daemon -p 1234
	// // add peer
	// curl -X POST -H "Content-Type: application/json" -d '{"Addr":"127.0.0.1:2"}' http://127.0.0.1:7122/peer

	// // get/add assets
	// curl http://127.0.0.1:7122/asset
	// curl -X POST -H "Content-Type: application/json" -d '{"Key":"a", "Value":3, "Price": 4}' http://127.0.0.1:7122/asset

	// // get balance
	// curl http://127.0.0.1:7122/balance

	// // get blockchain
	// curl http://127.0.0.1:7122/blockchain

	// // post mpcCalculate
	// curl -X POST -H "Content-Type: application/json" -d '{"Expr":"a+b", "Budget":10}' http://127.0.0.1:7122/mpcCalculate

}
