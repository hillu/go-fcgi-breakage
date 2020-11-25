# NGINX + Go `net/http/fcgi` breakage

This is a description of an error condition I ran into when trying to
hook a Go `net/http/fcgi`-based FastCGI application server into nginx,
together with a workaround and a solution:

(Found with Go 1.14-2~bpo10+1, nginx 1.14.2-2+deb10u3 on Debian
buster + buster-backports.)

## Demonstrating the problem

- Start nginx test instance that passes every HTTP request on
  `localhost:8080` to a FastCGI server on `localhost:8081`:
    ```
    $ sudo /usr/sbin/nginx -p . -c ./nginx.conf
	```
- Perform a HTTP request containing a body. Since the FastCGI server
  has not been started yet, a `502 Bad Gateway` error will be returned
  immediately:
    ```
    $ nc localhost 8080 << EOF
    > GET /foo HTTP/1.1
    > Host: localhost
    > Content-Length: 4
    > 
    > abcd
    > EOF
    HTTP/1.1 502 Bad Gateway
    Server: nginx/1.14.2
	Date: Wed, 25 Nov 2020 22:00:25 GMT
    Content-Type: text/html
    Content-Length: 173
    Connection: keep-alive
    
    <html>
    <head><title>502 Bad Gateway</title></head>
    <body bgcolor="white">
    <center><h1>502 Bad Gateway</h1></center>
    <hr><center>nginx/1.14.2</center>
    </body>
    </html>
    ```
- Start the minimal FastCGI application:
    ```
    $ go run fcgi-server.go
    ```
- Repeat the HTTP request from above. A 10s delay (equal to
  `fastcgi_read_timeout`) can be observed before the expected `404 Not
  Found` error is returned, even though the trivial "application code" has
  taken no delay to process the request:
    ```
    GET /foo HTTP/1.1
    Host: localhost
    Content-Length: 4
    
    abcd
    EOF
    HTTP/1.1 404 Not Found
    Server: nginx/1.14.2
    Content-Type: text/plain; charset=utf-8
    Transfer-Encoding: chunked
    Connection: keep-alive
    Date: Wed, 25 Nov 2020 22:00:57 GMT
    X-Content-Type-Options: nosniff

	```
- An error message indicating the timeout has been written to `error.log`:
    ```
	2020/11/25 23:01:07 [error] 27123#27123: *9 upstream timed out (110: Connection timed out) while reading upstream, client: 127.0.0.1, server: localhost, request: "GET /foo HTTP/1.1", upstream: "fastcgi://127.0.0.1:8081", host: "localhost"
	```

## Apparent easy workarounds

Setting either `fastcgi_keep_conn on;` or `fastcgi_buffering off;`
appears to make the problem go away.

## Analysis

The [FastCGI 1.0 Specification](https://www.mit.edu/~yandros/doc/specs/fcgi-spec.html)
states the following about the `FCGI_KEEP_CONN` flag:

> flags & FCGI_KEEP_CONN: If zero, the application closes the
> connection after responding to this request. If not zero, the
> application does not close the connection after responding to this
> request; the Web server retains responsibility for the connection.

Using Wireshark, we do not observe the`net/http/fcgi` server closing
the connection. This is curious, because `child.serveRequest` is
supposed to close the connection once it the `http.Handler` has
finished – and we had witnessed that our handler had returned
immediately.

Tracing reveals that two `FCGI_STDIN` messages have been transmitted,
but only one is properly processed by `child.handleRecord`. The
second, empty `FCGI_STDIN` message which indicates the end of the
stream is not recognized because the request id has been removed in
`child.serveRequest`, runningin a separate goroutine.  Since
`child.handleRecord` cannot detect the end of the "standard input"
stream, it will not close the pipe transporting request body data to
`child.serveRequest`, stalling it while it is trying to consume the
request body.

## Fix

Moving the lines of code that remove the request id from
`serveRequest` to `handleRecord` eliminates the race condition. The
`fcgi/` subdirectory contains a fixed version of `net/fcgi` from Go
1.14.2. Changing the corresponding import in `fcgi-server.go` can be
used to demonstrate the fix.

## Author

Hilko Bengen <<bengen@hilluzination.de>>
