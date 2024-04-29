package core

import (
	"errors"
	"strings"
)

type Errors []error

func NewErrors() *Errors {
	return &Errors{}
}

func ErrMessagesToErrors(errMessages []string) *Errors {
	if len(errMessages) == 0 {
		return &Errors{}
	}
	errs := &Errors{}
	for i := 0; i < len(errMessages); i++ {
		if errMessages[i] != "" {
			*errs = append(*errs, errors.New(errMessages[i]))
		}
	}
	return errs
}

func (p *Errors) Append(errs ...error) {
	for i := 0; i < len(errs); i++ {
		if errs[i] == nil {
			continue
		}
		*p = append(*p, errs[i])
	}
}

func (p *Errors) ToErrMessages() []string {
	if len(*p) == 0 {
		return []string{}
	}
	mapErrMessage := make(map[string]struct{}) // 过滤重复
	errMessages := []string{}
	for i := 0; i < len(*p); i++ {
		errMessage := (*p)[i].Error()
		if _, ok := mapErrMessage[errMessage]; ok {
			continue
		}
		errMessages = append(errMessages, errMessage)
		mapErrMessage[errMessage] = struct{}{}
	}
	return errMessages
}

func (p *Errors) Error() error {
	if len(*p) == 0 {
		return nil
	}
	return (*p)[0]
}

func (p *Errors) PrintError() error {
	if len(*p) == 0 {
		return nil
	}
	return errors.New(strings.TrimSpace(strings.Join(p.ToErrMessages(), "; ")))
}
