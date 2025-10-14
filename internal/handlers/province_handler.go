package handlers

import (
	"datapemilu2024/internal/repository"
	"datapemilu2024/templates"
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
)

type ProvinceHandler struct {
	repo *repository.ProvinceRepository
}

func NewProvinceHandler(repo *repository.ProvinceRepository) *ProvinceHandler {
	return &ProvinceHandler{repo: repo}
}

func (h *ProvinceHandler) Index(c echo.Context) error {
	provinces, err := h.repo.GetAllProvinces()
	if err != nil {
		log.Printf("Error fetching provinces: %v", err)
		return c.String(http.StatusInternalServerError, "Error fetching data")
	}

	component := templates.ProvincePage(provinces)
	return component.Render(c.Request().Context(), c.Response().Writer)
}
