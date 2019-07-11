# shclisem
Simple HTTP Client Semaphore

This is a Go library that uses a weighted semaphore from the x/sync package to manage concurent use of a http client.  Built to be a quick drop in replacement for the native http client. After the RequestHandler is built, create your http request struct, send it to the RequestHandler via the Do or DoWeighted methods, and bobs your uncle.

Added usability to get how much pressure is on the semaphore.  Uses 2 rwmutexes to guard the waiting and currently running weight counters.  Access the counters through the exported funcitons.
