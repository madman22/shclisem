package shclisem

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/sync/semaphore"
)

/*
Simple HTTP Client Semaphore

For limiting concurrent client connections using weighting.
Use small weights for lightweight requests and larger weights for large requests/responses
*/
type RequestHandler struct {
	client  *http.Client
	sem     *semaphore.Weighted
	timeout time.Duration
}

//Creates new RequestHandler with the given total weight, per item wait timeout, and http client, or the default http client if nil
func NewRequestHandler(tweight int, timeout time.Duration, cli *http.Client) *RequestHandler {
	var rh RequestHandler
	if tweight < 1 {
		tweight = 1
	}
	rh.sem = semaphore.NewWeighted(int64(tweight))
	if timeout < 1*time.Second { //check to make sure the timeout isn't too short
		timeout = 1 * time.Minute //set the default value for the per item timeout, 1 minute
	}
	rh.timeout = timeout
	if cli == nil {
		rh.client = http.DefaultClient
	} else {
		rh.client = cli
	}
	return &rh
}

//Tries to acquire semaphore using weight of 1, then runs the request on the http client
func (rh *RequestHandler) Do(req *http.Request) (*http.Response, error) {
	return rh.DoWeighted(req, 1)
}

//Tries to acquire semaphore using the given weight and default timeout, then runs the request on the http client
func (rh *RequestHandler) DoWeighted(req *http.Request, weight int) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(context.Background(), rh.timeout)
	defer cancel()
	return rh.DoWeightedContext(req, weight, ctx)
}

//Tries to acquire semaphore using the given weight and context, then runs the request on the http client
func (rh *RequestHandler) DoWeightedContext(req *http.Request, weight int, ctx context.Context) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("Request is nil")
	}
	if err := rh.checkStruct(); err != nil {
		return nil, err
	}
	if err := rh.sem.Acquire(ctx, int64(weight)); err != nil {
		return nil, err
	}
	defer rh.sem.Release(int64(weight))
	return rh.client.Do(req)
}

func (rh *RequestHandler) checkStruct() error {
	if rh.client == nil {
		return errors.New("nil http client, use the function NewRequestHandler to build the RequestHandler struct")
	}
	if rh.sem == nil {
		return errors.New("nil semaphore, use the function NewRequestHandler to build the RequestHandler struct")
	}
	return nil
}
