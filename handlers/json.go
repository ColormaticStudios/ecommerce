package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/gin-gonic/gin"
)

func bindStrictJSON(c *gin.Context, target any) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return io.EOF
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	if err := decoder.Decode(&struct{}{}); err != nil && !errors.Is(err, io.EOF) {
		return errors.New("request body must contain a single JSON object")
	}
	return nil
}
