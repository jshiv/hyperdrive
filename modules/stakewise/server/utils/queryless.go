package utils

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/goccy/go-json"

	"github.com/gorilla/mux"
	"github.com/nodeset-org/hyperdrive/modules/common/server"
	"github.com/nodeset-org/hyperdrive/modules/common/services"
)

type ApiResponse[Data any] struct {
	Data *Data `json:"data"`
}

type SuccessData struct {
	Success bool `json:"success"`
}

// Wrapper for callbacks used by call runners that simply run without following a structured pattern of
// querying the chain. This is the most general form of context and can be used by anything as it doesn't
// add any scaffolding.
// Structs implementing this will handle the caller-specific functionality.
type IQuerylessCallContext[DataType any] interface {
	// Prepare the response data in whatever way the context needs to do
	//PrepareData(data *DataType, opts *bind.TransactOpts) error
	PrepareData(data *DataType, opts *bind.TransactOpts) error
}

// Interface for queryless call context factories that handle GET calls.
// These will be invoked during route handling to create the unique context for the route.
type IQuerylessGetContextFactory[ContextType IQuerylessCallContext[DataType], DataType any] interface {
	// Create the context for the route
	Create(args url.Values) (ContextType, error)
}

// Interface for queryless call context factories that handle POST requests.
// These will be invoked during route handling to create the unique context for the route
type IQuerylessPostContextFactory[ContextType IQuerylessCallContext[DataType], BodyType any, DataType any] interface {
	// Create the context for the route
	Create(body BodyType) (ContextType, error)
}

// Registers a new route with the router, which will invoke the provided factory to create and execute the context
// for the route when it's called via GET; use this for typical general-purpose calls
func RegisterQuerylessGet[ContextType IQuerylessCallContext[DataType], DataType any](
	router *mux.Router,
	functionName string,
	factory IQuerylessGetContextFactory[ContextType, DataType],
	serviceProvider *services.ServiceProvider,
) {
	router.HandleFunc(fmt.Sprintf("/%s", functionName), func(w http.ResponseWriter, r *http.Request) {
		// Log
		args := r.URL.Query()
		isDebug := serviceProvider.IsDebugMode()
		log := serviceProvider.GetApiLogger()
		if isDebug {
			log.Printlnf("[%s] => %s", r.Method, r.URL.String())
		} else {
			log.Printlnf("[%s] => %s", r.Method, r.URL.Path)
		}

		// Check the method
		if r.Method != http.MethodGet {
			server.HandleInvalidMethod(log, w)
			return
		}

		// Create the handler and deal with any input validation errors
		context, err := factory.Create(args)
		if err != nil {
			server.HandleInputError(log, w, err)
			return
		}

		// Run the context's processing routine
		response, err := runQuerylessRoute[DataType](context, serviceProvider)
		server.HandleResponse(log, w, response, err, isDebug)
	})
}

// Registers a new route with the router, which will invoke the provided factory to create and execute the context
// for the route when it's called via POST; use this for typical general-purpose calls
func RegisterQuerylessPost[ContextType IQuerylessCallContext[DataType], BodyType any, DataType any](
	router *mux.Router,
	functionName string,
	factory IQuerylessPostContextFactory[ContextType, BodyType, DataType],
	serviceProvider *services.ServiceProvider,
) {
	router.HandleFunc(fmt.Sprintf("/%s", functionName), func(w http.ResponseWriter, r *http.Request) {
		// Log
		log := serviceProvider.GetApiLogger()
		isDebug := serviceProvider.IsDebugMode()
		log.Printlnf("[%s] => %s", r.Method, r.URL.Path)

		// Check the method
		if r.Method != http.MethodPost {
			server.HandleInvalidMethod(log, w)
			return
		}

		// Read the body
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			server.HandleInputError(log, w, fmt.Errorf("error reading request body: %w", err))
			return
		}
		if isDebug {
			log.Printlnf("BODY: %s", string(bodyBytes))
		}

		// Deserialize the body
		var body BodyType
		err = json.Unmarshal(bodyBytes, &body)
		if err != nil {
			server.HandleInputError(log, w, fmt.Errorf("error deserializing request body: %w", err))
			return
		}

		// Create the handler and deal with any input validation errors
		context, err := factory.Create(body)
		if err != nil {
			server.HandleInputError(log, w, err)
			return
		}

		// Run the context's processing routine
		response, err := runQuerylessRoute[DataType](context, serviceProvider)
		server.HandleResponse(log, w, response, err, isDebug)
	})
}

// Run a route registered with no structured chain query pattern
func runQuerylessRoute[DataType any](ctx IQuerylessCallContext[DataType], serviceProvider *services.ServiceProvider) (*ApiResponse[DataType], error) {
	// Get the services

	//TODO
	/*
		w := serviceProvider.GetWallet()

		// Get the transact opts if this node is ready for transaction
		var opts *bind.TransactOpts
		walletStatus, err := w.GetStatus()
		if err != nil {
			return nil, fmt.Errorf("error getting wallet status: %w", err)
		}
		if utils.IsWalletReady(walletStatus) {
			var err error
			opts, err = w.GetTransactor()
			if err != nil {
				return nil, fmt.Errorf("error getting node account transactor: %w", err)
			}
		}
	*/

	// Create the response and data
	data := new(DataType)
	response := &ApiResponse[DataType]{
		Data: data,
	}

	// Prep the data with the context-specific behavior
	err := ctx.PrepareData(data, nil) // opts)
	if err != nil {
		return nil, err
	}

	// Return
	return response, nil
}
