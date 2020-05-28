package telebot

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Raw lets you call any method of Bot API manually.
// It also handles API errors, so you only need to unwrap
// result field from json data.
func (b *Bot) Raw(method string, payload interface{}) ([]byte, error) {
	url := b.URL + "/bot" + b.Token + "/" + method

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, err
	}

	resp, err := b.client.Post(url, "application/json", &buf)
	if err != nil {
		return nil, errors.Wrap(err, "http.Post failed")
	}
	resp.Close = true
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError(err)
	}

	// returning data as well
	return data, extractOk(data)
}

func (b *Bot) sendFiles(method string, files map[string]File, params map[string]string) ([]byte, error) {
	rawFiles := map[string]interface{}{}
	for name, f := range files {
		switch {
		case f.InCloud():
			params[name] = f.FileID
		case f.FileURL != "":
			params[name] = f.FileURL
		case f.OnDisk():
			rawFiles[name] = f.FileLocal
		case f.FileReader != nil:
			rawFiles[name] = f.FileReader
		default:
			return nil, errors.Errorf("telebot: File for field %s doesn't exist", name)
		}
	}

	if len(rawFiles) == 0 {
		return b.Raw(method, params)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	for field, file := range rawFiles {
		if err := addFileToWriter(writer, params["file_name"], field, file); err != nil {
			return nil, wrapError(err)
		}
	}
	for field, value := range params {
		if err := writer.WriteField(field, value); err != nil {
			return nil, wrapError(err)
		}
	}
	if err := writer.Close(); err != nil {
		return nil, wrapError(err)
	}

	url := b.URL + "/bot" + b.Token + "/" + method

	resp, err := b.client.Post(url, writer.FormDataContentType(), &buf)
	if err != nil {
		return nil, wrapError(err)
	}
	resp.Close = true
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusInternalServerError {
		return nil, ErrInternal
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, wrapError(err)
	}

	return data, extractOk(data)
}

func addFileToWriter(writer *multipart.Writer, filename, field string, file interface{}) error {
	var reader io.Reader
	if r, ok := file.(io.Reader); ok {
		reader = r
	} else if path, ok := file.(string); ok {
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		reader = f
	} else {
		return errors.Errorf("telebot: File for field %v should be an io.ReadCloser or string", field)
	}

	part, err := writer.CreateFormFile(field, filename)
	if err != nil {
		return err
	}

	_, err = io.Copy(part, reader)
	return err
}

func (b *Bot) sendText(to Recipient, text string, opt *SendOptions) (*Message, error) {
	params := map[string]string{
		"chat_id": to.Recipient(),
		"text":    text,
	}
	embedSendOptions(params, opt)

	data, err := b.Raw("sendMessage", params)
	if err != nil {
		return nil, err
	}

	return extractMessage(data)
}

func (b *Bot) sendObject(f *File, what string, params map[string]string, files map[string]File) (*Message, error) {
	sendWhat := "send" + strings.Title(what)

	if what == "videoNote" {
		what = "video_note"
	}

	sendFiles := map[string]File{what: *f}
	for k, v := range files {
		sendFiles[k] = v
	}

	data, err := b.sendFiles(sendWhat, sendFiles, params)
	if err != nil {
		return nil, err
	}

	return extractMessage(data)
}

func (b *Bot) getMe() (*User, error) {
	data, err := b.Raw("getMe", nil)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result *User
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, wrapError(err)
	}
	return resp.Result, nil

}

func (b *Bot) getUpdates(offset, limit int, timeout time.Duration, allowed []string) ([]Update, error) {
	params := map[string]string{
		"offset":  strconv.Itoa(offset),
		"timeout": strconv.Itoa(int(timeout / time.Second)),
	}

	if limit != 0 {
		params["limit"] = strconv.Itoa(limit)
	}
	if len(allowed) > 0 {
		data, _ := json.Marshal(allowed)
		params["allowed_updates"] = string(data)
	}

	data, err := b.Raw("getUpdates", params)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result []Update
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, wrapError(err)
	}
	return resp.Result, nil
}
