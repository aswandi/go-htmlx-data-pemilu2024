package handlers

import (
	"datapemilu2024/internal/repository"
	"datapemilu2024/templates"

	"github.com/labstack/echo/v4"
)

type KabupatenHandler struct {
	repo *repository.KabupatenRepository
}

func NewKabupatenHandler(repo *repository.KabupatenRepository) *KabupatenHandler {
	return &KabupatenHandler{repo: repo}
}

func (h *KabupatenHandler) ShowKabupaten(c echo.Context) error {
	provinceCode := c.Param("code")

	kabupatens, err := h.repo.GetKabupatenByProvinceCode(provinceCode)
	if err != nil {
		return c.String(500, err.Error())
	}

	provinceName := ""
	if len(kabupatens) > 0 {
		provinceName = kabupatens[0].ProvinceName
	}

	component := templates.KabupatenPage(kabupatens, provinceCode, provinceName)
	return component.Render(c.Request().Context(), c.Response().Writer)
}
