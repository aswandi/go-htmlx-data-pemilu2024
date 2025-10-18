package main

import (
	"datapemilu2024/internal/database"
	"datapemilu2024/internal/handlers"
	"datapemilu2024/internal/repository"
	"log"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Database configuration
	dbConfig := database.Config{
		Host:     "127.0.0.1",
		Port:     "3306",
		Username: "root",
		Password: "YOUR_DATABASE_PASSWORD", // TODO: Load from environment variable
		Database: "pileg2024",
	}

	// Connect to database
	if err := database.Connect(dbConfig); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer database.Close()

	// Initialize repositories
	provinceRepo := repository.NewProvinceRepository(database.DB)
	kabupatenRepo := repository.NewKabupatenRepository(database.DB)
	dapilRepo := repository.NewDapilRepository(database.DB)

	// Initialize handlers
	provinceHandler := handlers.NewProvinceHandler(provinceRepo)
	kabupatenHandler := handlers.NewKabupatenHandler(kabupatenRepo)
	dapilHandler := handlers.NewDapilHandler(dapilRepo)
	dprDownloadHandler := handlers.NewDPRDownloadHandler(database.DB)
	changelogHandler := handlers.NewChangelogHandler()

	// Initialize Echo
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	// Routes
	e.GET("/", provinceHandler.Index)
	e.GET("/changelog", changelogHandler.Index)
	e.GET("/provinsi/:code", kabupatenHandler.ShowKabupaten)
	e.GET("/dapil/:code", dapilHandler.ShowDapil)
	e.GET("/download/dpr-ri-caleg/:id", dprDownloadHandler.DownloadDPRRICaleg)
	e.GET("/download/dpr-ri-partai/:id", dprDownloadHandler.DownloadDPRRIPartai)
	e.GET("/download/provinsi/dpr-ri-partai/:code", dprDownloadHandler.DownloadProvinsiDPRRIPartai)
	e.GET("/download/provinsi/dpr-ri-partai-tps/:code", dprDownloadHandler.DownloadProvinsiDPRRIPartaiTPS)
	e.GET("/download/provinsi/dpr-ri-caleg/:code", dprDownloadHandler.DownloadProvinsiDPRRICaleg)
	e.GET("/download/provinsi/dpr-ri-caleg-tps/:code", dprDownloadHandler.DownloadProvinsiDPRRICalegTPS)
	e.GET("/download/provinsi/pilpres/:id", dprDownloadHandler.DownloadPilpresByProvince)
	e.GET("/download/provinsi/dpd/:code", dprDownloadHandler.DownloadProvinsiDPD)
	e.GET("/download/provinsi/dpd-tps/:id", dprDownloadHandler.DownloadDPDByProvinceTPS)
	e.GET("/download/provinsi/dprd-prov-partai/:code", dprDownloadHandler.DownloadProvinsiDPRDProvPartai)
	e.GET("/download/provinsi/dprd-prov-caleg/:code", dprDownloadHandler.DownloadProvinsiDPRDProvCaleg)
	e.GET("/download/kabupaten/dpr-ri-partai-tps/:code", dprDownloadHandler.DownloadKabupatenDPRRIPartaiTPS)
	e.GET("/download/kabupaten/dpr-ri-caleg-tps/:code", dprDownloadHandler.DownloadKabupatenDPRRICalegTPS)
	// Dapil download routes
	e.GET("/download/dapil/pilpres/:code", dprDownloadHandler.DownloadDapilPilpres)
	e.GET("/download/dapil/dpr-ri-partai/:code", dprDownloadHandler.DownloadDapilDPRRIPartai)
	e.GET("/download/dapil/dpr-ri-caleg/:code", dprDownloadHandler.DownloadDapilDPRRICaleg)
	e.GET("/download/dapil/dpr-ri-partai-tps/:code", dprDownloadHandler.DownloadDapilDPRRIPartaiTPS)
	e.GET("/download/dapil/dpr-ri-caleg-tps/:code", dprDownloadHandler.DownloadDapilDPRRICalegTPS)
	// DPRD Kabupaten download routes
	e.GET("/download/dprd-kab-partai/:code", dprDownloadHandler.DownloadDPRDKabPartai)
	e.GET("/download/dprd-kab-caleg/:id", dprDownloadHandler.DownloadDPRDKabCalegByProvince)

	// Start server
	log.Println("Server starting on :8080")
	if err := e.Start(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
