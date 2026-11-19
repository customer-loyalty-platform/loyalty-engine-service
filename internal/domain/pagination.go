package domain

type PaginationQuery struct {
	Page   int    `form:"page,default=1" binding:"min=1"`
	Limit  int    `form:"limit,default=10" binding:"min=1,max=100"`
	Search string `form:"search"`
}

func (p PaginationQuery) Offset() int {
	if p.Page <= 0 {
		return 0
	}
	return (p.Page - 1) * p.Limit
}

