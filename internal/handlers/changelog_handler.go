package handlers

import (
	"datapemilu2024/templates"

	"github.com/labstack/echo/v4"
)

type ChangelogHandler struct{}

func NewChangelogHandler() *ChangelogHandler {
	return &ChangelogHandler{}
}

func (h *ChangelogHandler) Index(c echo.Context) error {
	component := templates.ChangelogPage()
	return component.Render(c.Request().Context(), c.Response().Writer)
}
