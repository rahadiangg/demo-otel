package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
)

const name = "payment-svc"

var port = "8080"
var otel_host = "127.0.0.1:4317"
var sampler = float64(1)

func init() {

	e_port, exist := os.LookupEnv("PORT")
	if exist {
		port = e_port
	}

	e_otel_host, exist := os.LookupEnv("OTEL_HOST")
	if exist {
		otel_host = e_otel_host
	}

	e_sampler, exist := os.LookupEnv("OTEL_SAMPLER_RATIO")
	if exist {
		e_sampler_float, err := strconv.ParseFloat(e_sampler, 64)
		if err != nil {
			log.Panic(err)
		}

		sampler = e_sampler_float
	}
}
func main() {

	// ctx, cancel := context.WithCancel(context.Background())
	ctx := context.Background()

	// Trace Provider
	tp, err := initTraceProvider()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		// Do not make the application hang when it is tp.
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// Gin
	r := gin.Default()
	r.Use(otelgin.Middleware(name)) // middleware otelgin

	r.POST("/balance-check", func(c *gin.Context) {

		// parse baggage
		reqBaggage := baggage.FromContext(c.Request.Context())
		log.Printf("user ID \t\t: %s\n", reqBaggage.Member("user_id").Value())
		log.Printf("mock baggages \t: %s\n", reqBaggage.Member("test_baggages").Value())

		ctx, span := tp.Tracer(name).Start(c.Request.Context(), "parse body to json")
		defer span.End()

		var userData BalanceRequest
		if err = c.BindJSON(&userData); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "error when parse body to json")
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "error when parse body to json",
			})
			return
		}

		// mmock check user ID exist or not
		ctx, span = tp.Tracer(name).Start(ctx, "cek user ID")
		defer span.End()
		if userData.UserId != 1 {
			span.RecordError(err)
			span.SetStatus(codes.Error, "user ID not found")
			c.JSON(http.StatusNotFound, gin.H{
				"message": "user ID not found",
			})
			return
		}
		// Do work cek user id.....

		// mock balance (just for demo)
		ctx, span = tp.Tracer(name).Start(ctx, "cek balance")
		defer span.End()

		var balance int64 = 100000

		// return response
		_, span = tp.Tracer(name).Start(ctx, "return response")
		defer span.End()

		var jsonResponse = BalanceResponse{
			Balance: balance,
		}
		span.SetAttributes(attribute.Int64("balance", balance))

		c.JSON(http.StatusOK, jsonResponse)

	})

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: r,
	}

	// run server
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// shutdown server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown server ....")

	ctxServer, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxServer); err != nil {
		log.Fatal("Shutdown server:", err)
	}

	log.Println("Server exiting")
}
