package extractor

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"google.golang.org/grpc/metadata"
)

type Extractor interface {
	Get(ctx context.Context, name string) []string
	GetFirst(ctx context.Context, name string) string
	GetTokenID(ctx context.Context) string
	GetTenantID(ctx context.Context) string
	GetUserID(ctx context.Context) (int64, error)
	GetSafeID(ctx context.Context) (string, bool)
	GetRoleIDs(ctx context.Context) []string
	GetGroupIDs(ctx context.Context) []string
	GetXForwardedFor(ctx context.Context) string
	GetUtmSource(ctx context.Context) string
	GetPhoneNumber(ctx context.Context) string
	GetLabelIDs(ctx context.Context) []string
	GetLastTenSignInDate(ctx context.Context) []string
	GetXTotalDeposit(ctx context.Context) string
	GetXTotalWithdraw(ctx context.Context) string
	GetAppID(ctx context.Context) string
}

type extractor struct {
}

func New() Extractor {
	return &extractor{}
}

func (t *extractor) Get(ctx context.Context, name string) []string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil
	}

	return md.Get(name)
}

func (t *extractor) GetFirst(ctx context.Context, name string) string {
	values := t.Get(ctx, name)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (t *extractor) GetTokenID(ctx context.Context) string {
	values := t.Get(ctx, TokenID)
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (t *extractor) GetTenantID(ctx context.Context) string {
	values := t.Get(ctx, TenantID)
	if len(values) == 0 {
		// Return empty in case TenantID is undefined
		// Empty is default TenantID
		return ""
	}

	return values[0]
}

func (t *extractor) GetUserID(ctx context.Context) (int64, error) {
	values := t.Get(ctx, UserID)
	if len(values) == 0 {
		return 0, errors.New("metadata does not have x-user-id")
	}

	return strconv.ParseInt(values[0], 10, 64)
}

func (t *extractor) GetSafeID(ctx context.Context) (string, bool) {
	values := t.Get(ctx, SafeID)
	if len(values) == 0 {
		return "", false
	}

	return values[0], true
}

func (t *extractor) GetRoleIDs(ctx context.Context) []string {
	return t.Get(ctx, RoleID)
}

func (t *extractor) GetGroupIDs(ctx context.Context) []string {
	return t.Get(ctx, GroupID)
}

func (t *extractor) GetXForwardedFor(ctx context.Context) string {
	values := t.Get(ctx, XForwardedFor)
	if len(values) == 0 {
		return ""
	}

	return strings.Join(values[:], ",")
}

func (t *extractor) GetUtmSource(ctx context.Context) string {
	values := t.Get(ctx, XUtmSource)
	if len(values) == 0 {
		return ""
	}

	return strings.Join(values[:], ",")
}

func (t *extractor) GetPhoneNumber(ctx context.Context) string {
	return t.GetFirst(ctx, XPhoneNumber)
}

func (t *extractor) GetLabelIDs(ctx context.Context) []string {
	return t.Get(ctx, XLabelIDs)
}

func (t *extractor) GetLastTenSignInDate(ctx context.Context) []string {
	return t.Get(ctx, XLastTenSignInDate)
}

func (t *extractor) GetXTotalDeposit(ctx context.Context) string {
	return t.GetFirst(ctx, XTotalDeposit)
}

func (t *extractor) GetXTotalWithdraw(ctx context.Context) string {
	return t.GetFirst(ctx, XTotalWithdraw)
}

func (t *extractor) GetAppID(ctx context.Context) string {
	return t.GetFirst(ctx, XAppID)
}