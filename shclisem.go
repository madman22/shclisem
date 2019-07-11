package shclisem

import (
	"context"
	"errors"

	"math"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"
)

/*
Simple HTTP Client Semaphore

For limiting concurrent client connections using weighting.  Use the NewRequestHandler function to build the struct.
*/
type RequestHandler struct {
	client      *http.Client
	sem         *semaphore.Weighted
	timeout     time.Duration
	current     int
	currentlock sync.RWMutex
	waiting     int
	waitlock    sync.RWMutex
}

//Creates new RequestHandler with the given total weight, per item wait timeout, and http client, or the default http client if nil
func NewRequestHandler(tweight int, timeout time.Duration, cli *http.Client) *RequestHandler {
	var rh RequestHandler
	if tweight < 1 {
		tweight = 1
	} else if tweight > math.MaxInt32 {
		tweight = math.MaxInt32
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

//Tries to acquire semaphore using weight of 1 and default timeout, then runs the request on the http client
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
	if weight > math.MaxInt32 {
		return nil, errors.New("Too heavy, drop some weight")
	}
	if err := rh.checkStruct(); err != nil {
		return nil, err
	}
	rh.waitlock.Lock()
	if rh.waiting >= math.MaxInt32-weight {
		rh.waitlock.Unlock()
		return nil, errors.New("Too much waiting! Slow your roll!")
	} else {
		rh.waiting += weight
		rh.waitlock.Unlock()
	}
	if err := rh.sem.Acquire(ctx, int64(weight)); err != nil {
		rh.waitlock.Lock()
		rh.waiting -= weight
		rh.waitlock.Unlock()
		return nil, err
	}
	rh.waitlock.Lock()
	rh.waiting -= weight
	rh.waitlock.Unlock()
	rh.currentlock.Lock()
	rh.current += weight
	rh.currentlock.Unlock()
	defer rh.sem.Release(int64(weight))
	defer func() {
		rh.currentlock.Lock()
		rh.current -= weight
		rh.currentlock.Unlock()
	}()
	return rh.client.Do(req)
}

//Returns the amount of weight waiting for the semaphore
func (rh *RequestHandler) GetWaitingWeight() int {
	rh.waitlock.RLock()
	defer rh.waitlock.RUnlock()
	return rh.waiting
}

//Returns the used weight that has aquired the semaphore
func (rh *RequestHandler) GetCurrentWeight() int {
	rh.currentlock.RLock()
	defer rh.currentlock.RUnlock()
	return rh.current
}

//verifies the struct was built properly
func (rh *RequestHandler) checkStruct() error {
	if rh.client == nil {
		return errors.New("nil http client, use the function NewRequestHandler to build the RequestHandler struct")
	}
	if rh.sem == nil {
		return errors.New("nil semaphore, use the function NewRequestHandler to build the RequestHandler struct")
	}
	return nil
}
