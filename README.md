# shclisem
Simple HTTP Client Semaphore

This is a Go library that uses a weighted semaphore from the x/sync package to manage concurent use of a http client.  Built to be a quick drop in replacement for the native http client. After the RequestHandler is built, create your http request struct, send it to the RequestHandler via the Do or DoWeighted methods, and bobs your uncle.

