package sort

import (
	"errors"
	"fmt"

	pb "irelia/api"
)

func Contains[T comparable](s []T, e T) bool {
	for _, v := range s {
		if v == e {
			return true
		}
	}
	return false
}

func GetSort(columns []string, table string, sorts []*pb.SortMethod) ([]string, error) {
	var values []string
	for _, data := range sorts {
		if Contains(columns, data.Name) {
            switch data.Type {
            case pb.SortType_SORT_TYPE_ASC:
                values = append(values, fmt.Sprintf("`%s`.`%s` ASC", table, data.Name))
            case pb.SortType_SORT_TYPE_DESC:
                values = append(values, fmt.Sprintf("`%s`.`%s` DESC", table, data.Name))
            }
            continue
        }
		return nil, errors.New("column not found")
	}
	return values, nil
}