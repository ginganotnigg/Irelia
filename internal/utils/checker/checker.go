package checker

import (
	"context"
	"fmt"
)

func CheckRole(ctx context.Context, targetRole string, roleIds []string) error {
	if len(roleIds) != 1 {
		return fmt.Errorf("invalid role id")
	}

	if roleIds[0] != targetRole {
		return fmt.Errorf("permission denied")
	}

	return nil
}