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

//TODO implement retry structure
//		retry every x duration
//		every error or only timeouts?

/*
Simple HTTP Client Semaphore

For limiting concurrent client connections using weighting.  Use the NewRequestHandler function to build the struct.
*/
type RequestHandler struct {
	client  *http.Client
	sem     *semaphore.Weighted
	timeout time.Duration
	current *counter
	waiting *counter
	total   *counter
	errs    *counter
}

type counter struct {
	name     string
	count    int
	max      int
	rollover bool
	countmux sync.RWMutex
}

func newCounter(name string, max int, rollover bool) *counter {
	var c counter
	if max == 0 {
		c.max = math.MaxInt64
	} else {
		c.max = max
	}
	c.name = name
	c.rollover = rollover
	return &c
}

func (c *counter) Add() error {
	c.countmux.Lock()
	defer c.countmux.Unlock()
	if c.count == c.max {
		if c.rollover {
			c.count = 0
		} else {
			return errors.New(c.name + " count too high.  Slow your roll!")
		}
	}
	c.count++
	return nil
}

func (c *counter) Remove() error {
	c.countmux.Lock()
	defer c.countmux.Unlock()
	if c.count == 0 {
		if c.rollover {
			return nil
		} else {
			return errors.New("Too much removal")
		}
	}
	c.count--
	return nil
}

func (c *counter) Count() int {
	c.countmux.RLock()
	defer c.countmux.RUnlock()
	return c.count
}

func (c *counter) Max() int {
	return c.max
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
	} else if timeout > time.Hour {
		timeout = 1 * time.Minute
	}
	rh.timeout = timeout
	if cli == nil {
		rh.client = http.DefaultClient
	} else {
		rh.client = cli
	}
	rh.current = newCounter("Running", 0, false)
	rh.waiting = newCounter("Waiting", 0, false)
	rh.total = newCounter("Total", 0, true)
	rh.errs = newCounter("Errors", 0, true)
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
	if weight < 1 {
		weight = 1
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := rh.checkStruct(); err != nil {
		return nil, err
	}
	if err := rh.waiting.Add(); err != nil {
		return nil, err
	}
	if err := rh.sem.Acquire(ctx, int64(weight)); err != nil {
		if errr := rh.waiting.Remove(); err != nil {
			return nil, errors.New(errr.Error() + " & " + err.Error())
		}
		return nil, err
	}
	defer rh.sem.Release(int64(weight))
	if err := rh.waiting.Remove(); err != nil {
		return nil, err
	}
	if err := rh.current.Add(); err != nil {
		return nil, err
	}
	resp, err := rh.client.Do(req)
	if errr := rh.current.Remove(); errr != nil {
		rh.errs.Add()
		return nil, errors.New(errr.Error() + " & " + err.Error())
	}
	if err != nil {
		return nil, err
	}
	rh.total.Add()
	return resp, nil
}

//Returns the amount of weight waiting for the semaphore
func (rh *RequestHandler) GetWaitingWeight() int {
	if rh.waiting == nil {
		return 0
	}
	return rh.waiting.Count()
}

//Returns the used weight that has aquired the semaphore
func (rh *RequestHandler) GetCurrentWeight() int {
	if rh.current == nil {
		return 0
	}
	return rh.current.Count()
}

//Returns the number of http request errors
func (rh *RequestHandler) GetErrorCount() int {
	if rh.errs == nil {
		return 0
	}
	return rh.errs.Count()
}

//Returns the number of successful requests
func (rh *RequestHandler) GetTotalCount() int {
	if rh.total == nil {
		return 0
	}
	return rh.total.Count()
}

//verifies the struct was built properly
func (rh *RequestHandler) checkStruct() error {
	if rh.client == nil {
		return errors.New("nil http client, use the function NewRequestHandler to build the RequestHandler struct")
	}
	if rh.sem == nil {
		return errors.New("nil semaphore, use the function NewRequestHandler to build the RequestHandler struct")
	}
	if rh.current == nil {
		return errors.New("nil current counter, use the function NewRequestHandler to build the RequestHandler struct")
	}
	if rh.waiting == nil {
		return errors.New("nil waiting counter, use the function NewRequestHandler to build the RequestHandler struct")
	}
	if rh.errs == nil {
		return errors.New("nil error counter, use the function NewRequestHandler to build the RequestHandler struct")
	}
	if rh.total == nil {
		return errors.New("nil total counter, use the function NewRequestHandler to build the RequestHandler struct")
	}
	return nil
}
