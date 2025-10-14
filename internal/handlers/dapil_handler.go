package handlers

import (
	"datapemilu2024/internal/repository"
	"datapemilu2024/templates"

	"github.com/labstack/echo/v4"
)

type DapilHandler struct {
	repo *repository.DapilRepository
}

func NewDapilHandler(repo *repository.DapilRepository) *DapilHandler {
	return &DapilHandler{repo: repo}
}

func (h *DapilHandler) ShowDapil(c echo.Context) error {
	provinceCode := c.Param("code")

	dapils, err := h.repo.GetDapilByProvinceCode(provinceCode)
	if err != nil {
		return c.String(500, err.Error())
	}

	provinceName := ""
	if len(dapils) > 0 {
		provinceName = dapils[0].ProvinceName
	}

	component := templates.DapilPage(dapils, provinceCode, provinceName)
	return component.Render(c.Request().Context(), c.Response().Writer)
}
