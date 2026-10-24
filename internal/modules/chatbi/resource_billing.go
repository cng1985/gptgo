package chatbi

import (
	"context"
	"strings"
	"unicode/utf8"

	apperr "gptgo/internal/platform/errors"
	"gptgo/internal/platform/response"
)

func (r *Resource) Wallet(ctx context.Context) (response.Response[WalletResponse], error) {
	var zero response.Response[WalletResponse]
	balance, err := r.repo.EnsureWallet(ctx)
	if err != nil {
		return zero, err
	}
	return response.OK(WalletResponse{Balance: balance, WelcomeCredits: WelcomeCredits, AskCost: AskCost}), nil
}

func (r *Resource) Topup(ctx context.Context, request *TopupRequest) (response.Response[TopupResponse], error) {
	var zero response.Response[TopupResponse]
	if request == nil {
		return zero, apperr.Invalid("参数格式错误")
	}
	if request.Amount < 1 || request.Amount > 100000 {
		return zero, apperr.Invalid("充值额度需要在 1 到 100000 之间")
	}
	note := strings.TrimSpace(request.Note)
	if note == "" {
		note = "账户充值"
	}
	if utf8.RuneCountInString(note) > 80 {
		return zero, apperr.Invalid("备注需要在 80 个字以内")
	}
	result, err := r.repo.Topup(ctx, request.Amount, note)
	if err != nil {
		return zero, err
	}
	return response.OK(result), nil
}

func (r *Resource) SearchBilling(ctx context.Context, search *BillingSearch) (response.PageResponse[BillingSimple], error) {
	var zero response.PageResponse[BillingSimple]
	if search == nil {
		search = &BillingSearch{}
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 20)
	if err != nil {
		return zero, err
	}
	search.Page, search.PageSize = page, size
	return r.repo.SearchBilling(ctx, search)
}

func (r *Resource) SearchUsage(ctx context.Context, search *UsageSearch) (response.PageResponse[UsageSimple], error) {
	var zero response.PageResponse[UsageSimple]
	if search == nil {
		search = &UsageSearch{}
	}
	page, size, err := normalizePage(search.Page, search.PageSize, 20)
	if err != nil {
		return zero, err
	}
	search.Page, search.PageSize = page, size
	if utf8.RuneCountInString(search.Keyword) > 100 {
		return zero, apperr.Invalid("搜索关键词过长")
	}
	return r.repo.SearchUsage(ctx, search)
}

func (r *Resource) Summarize(ctx context.Context, request *UsageSummaryRequest) (response.Response[UsageSummary], error) {
	var zero response.Response[UsageSummary]
	if request == nil {
		request = &UsageSummaryRequest{}
	}
	days := request.Days
	if days == 0 {
		days = 14
	}
	if days < 1 || days > 90 {
		return zero, apperr.Invalid("统计天数需要在 1 到 90 之间")
	}
	summary, err := r.repo.Summarize(ctx, days)
	if err != nil {
		return zero, err
	}
	if summary.Days == nil {
		summary.Days = []UsageDay{}
	}
	if summary.Datasources == nil {
		summary.Datasources = []UsageSource{}
	}
	return response.OK(summary), nil
}
