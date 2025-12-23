package testutil

import (
	"context"
	"testing"
	"time"

	"github.com/docker/go-connections/nat"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func MySQL(ctx context.Context, tb *testing.T) (tc.Container, string, int) {
	tb.Helper()
	defer func() {
		if r := recover(); r != nil {
			tb.Skipf("skipping: docker/testcontainers not available (%v)", r)
		}
	}()
	req := tc.ContainerRequest{
		Image:        "mysql:8.4",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_DATABASE":      "sms_gateway",
			"MYSQL_USER":          "sms_user",
			"MYSQL_PASSWORD":      "sms_pass",
			"MYSQL_ROOT_PASSWORD": "root_pass",
		},
		WaitingFor: wait.ForLog("port: 3306  MySQL Community Server").WithStartupTimeout(90 * time.Second),
	}
	mysqlC, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		tb.Skipf("skipping: docker/testcontainers not available (%v)", err)
	}
	host, err := mysqlC.Host(ctx)
	if err != nil {
		tb.Fatalf("mysql host: %v", err)
	}
	port, err := mysqlC.MappedPort(ctx, "3306")
	if err != nil {
		tb.Fatalf("mysql port: %v", err)
	}
	return mysqlC, host, port.Int()
}

func Rabbit(ctx context.Context, t *testing.T) (tc.Container, string, int) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("skipping: docker/testcontainers not available (%v)", r)
		}
	}()
	req := tc.ContainerRequest{
		Image:        "rabbitmq:3.13-management",
		ExposedPorts: []string{"5672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "rabbit_user",
			"RABBITMQ_DEFAULT_PASS": "rabbit_pass",
		},
		WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(90 * time.Second),
	}
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		t.Skipf("skipping: docker/testcontainers not available (%v)", err)
	}
	host, err := c.Host(ctx)
	if err != nil {
		t.Fatalf("rabbit host: %v", err)
	}
	port, err := c.MappedPort(ctx, nat.Port("5672/tcp"))
	if err != nil {
		t.Fatalf("rabbit port: %v", err)
	}
	return c, host, port.Int()
}
