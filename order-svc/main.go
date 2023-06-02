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
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"gorm.io/gorm/clause"
)

const name = "order-svc"

var port = "8080"
var db_host = "127.0.0.1"
var otel_host = "127.0.0.1"
var db_max_conn = "80"
var sampler = float64(1)

func init() {
	e_db_host, exist := os.LookupEnv("DB_HOST")
	if exist {
		db_host = e_db_host
	}

	e_port, exist := os.LookupEnv("PORT")
	if exist {
		port = e_port
	}

	e_otel_host, exist := os.LookupEnv("OTEL_HOST")
	if exist {
		otel_host = e_otel_host
	}

	e_db_max_conn, exist := os.LookupEnv("DB_MAX_CONN")
	if exist {
		db_max_conn = e_db_max_conn
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

	// database
	db := initDB()

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

	// Metric Provider
	mp, err := initMeterProvider(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()
		if err := mp.Shutdown(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	meter := mp.Meter(name)

	// Create conter metric
	apiCounter, err := meter.Int64Counter("api counter")
	if err != nil {
		log.Fatalf("can't initialize counter api hit: %v", err)
	}

	// Gin
	r := gin.Default()
	r.Use(otelgin.Middleware(name)) // middleware otelgin

	eventGroup := r.Group("/event")
	eventGroup.GET("/:id", func(c *gin.Context) {

		var data Event
		id := c.Param("id")

		ctx, span := tp.Tracer(name).Start(c.Request.Context(), "Query to DB")
		defer span.End()

		d := db.WithContext(ctx).First(&data, id)

		if d.Error != nil {
			span.SetStatus(codes.Error, "error get query")
			span.RecordError(d.Error)
			c.JSON(http.StatusInternalServerError, "error get query")
			return
		}

		span.AddEvent("request finish")

		c.JSON(http.StatusOK, gin.H{
			"data": data,
		})
	})

	eventGroup.POST("/:id/buy", func(c *gin.Context) {

		id := c.Param("id")
		var dataGet Event

		dbTx := db.Begin()

		// check remaning quota
		ctxQuota, spanQuota := tp.Tracer(name).Start(c.Request.Context(), "check remaning quota")
		defer spanQuota.End()

		tx := dbTx.Clauses(clause.Locking{Strength: "UPDATE"}).WithContext(ctxQuota).First(&dataGet, id) // locking
		if tx.Error != nil {
			dbTx.Rollback()
			spanQuota.RecordError(tx.Error)
			spanQuota.SetStatus(codes.Error, "error when get data for check remaining quota")
			c.JSON(http.StatusInternalServerError, tx.Error.Error())
			return
		}

		// sold out
		if dataGet.Quota <= 0 {
			dbTx.Rollback()
			c.JSON(http.StatusConflict, "tiket sold out")
			return
		}

		// if ticket still available
		ctxBuy, spanBuy := tp.Tracer(name).Start(ctxQuota, "buy a ticket")
		defer spanBuy.End()

		finalQuota := dataGet.Quota - 1 // decrease 1

		tx = dbTx.WithContext(ctxBuy).Model(&dataGet).Update("quota", finalQuota)
		if tx.Error != nil {
			dbTx.Rollback()
			spanBuy.RecordError(tx.Error)
			spanBuy.SetStatus(codes.Error, "error update ticket data")
			c.JSON(http.StatusInternalServerError, tx.Error.Error())
			return
		}

		// success
		dbTx.Commit()
		apiCounter.Add(c.Request.Context(), 1, metric.WithAttributes(
			attribute.String("method", c.Request.Method),
			attribute.String("endpoint", c.FullPath()),
			attribute.String("status", "success"),
		)) // increase meter
		c.JSON(http.StatusOK, "ok tiket berhasil dibeli")
	})

	r.GET("/", func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "Hola, ini order service")
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
