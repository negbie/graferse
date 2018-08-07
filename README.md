# graferse
### About

**graferse** is a simple http reverse proxy for Grafana. It will set a special header
(X-WEBAUTH-USER) with a configurable Username. With this you can use Grafana's Authproxy 
to bypass authentication just enable it inside the Grafana config.

```
[auth.proxy]
enabled = true
header_name = X-WEBAUTH-USER
header_property = username
```

The code is easy to read so you can easily extend it. For example you could pass a special
timestamp format and convert it to one which can be read by Grafana. You could also manipulate
template variables to provide some kind of SaaS.


#### Usage
```
  -cert string
        SSL certificate path
  -grafana_url string
        Grafana URL (default "http://localhost:3000")
  -grafana_user string
        Grafana Authproxy Username (default "admin")
  -httptest.serve string
        if non-empty, httptest.NewServer serves on this address and blocks
  -key string
        SSL private Key path
  -metric
        Expose prometheus metrics
  -proxy_addr string
        Reverse proxy listen address (default ":8080")
  -readonly
        Don't allow changes inside Grafana (default true)
  -timeout int
        HTTP read, write timeout in seconds (default 16)
```
