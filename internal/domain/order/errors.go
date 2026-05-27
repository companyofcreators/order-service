package order

import "errors"

var (
	ErrOrderNotFound         = errors.New("заказ не найден")
	ErrCategoryNotFound      = errors.New("категория не найдена")
	ErrReviewNotFound        = errors.New("отзыв не найден")
	ErrComplaintNotFound     = errors.New("жалоба не найдена")

	ErrInvalidTransition      = errors.New("недопустимый переход статуса")
	ErrOrderNotCancellable    = errors.New("заказ не может быть отменён в текущем статусе")
	ErrOrderNotCompletable    = errors.New("заказ не может быть завершён в текущем статусе")
	ErrNotOrderParticipant    = errors.New("пользователь не является участником этого заказа")
	ErrOrderNotCompleted      = errors.New("заказ ещё не завершён")
	ErrAlreadyReviewed        = errors.New("пользователь уже оставил отзыв на этот заказ")
	ErrInvalidRating          = errors.New("оценка должна быть от 1 до 5")
	ErrCategoryHasChildren    = errors.New("нельзя удалить категорию с подкатегориями")
	ErrCategorySlugExists     = errors.New("категория с таким slug уже существует")
	ErrComplaintNotModifiable = errors.New("жалоба не может быть изменена в текущем статусе")
	ErrInvalidComplaintStatus = errors.New("недопустимый переход статуса жалобы")

	ErrUnauthorized = errors.New("не авторизован")
	ErrForbidden    = errors.New("доступ запрещён")
)
