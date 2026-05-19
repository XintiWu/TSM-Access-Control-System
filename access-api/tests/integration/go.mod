module github.com/tsmc/access-api/tests/integration

go 1.22

require (
	github.com/gin-gonic/gin v1.10.0
	github.com/redis/go-redis/v9 v9.7.0
	github.com/testcontainers/testcontainers-go v0.33.0
	github.com/tsmc/access-api v0.0.0
)

replace github.com/tsmc/access-api => ../..
