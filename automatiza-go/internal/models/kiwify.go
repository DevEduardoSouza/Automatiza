package models

import "time"

// KiwifyWebhookPayload represents the incoming Kiwify webhook payload.
// Kiwify can send data in two formats:
// 1. With wrapper: { signature, order: { Customer, Product, ... } }
// 2. Direct in body: { Customer, Product, webhook_event_type, ... }
type KiwifyWebhookPayload struct {
	// Wrapper format fields
	Signature string           `json:"signature,omitempty"`
	Order     *KiwifyOrderData `json:"order,omitempty"`

	// Direct format fields (when order wrapper is not present)
	Customer         *KiwifyCustomer     `json:"Customer,omitempty"`
	Product          *KiwifyProduct      `json:"Product,omitempty"`
	Subscription     *KiwifySubscription `json:"Subscription,omitempty"`
	WebhookEventType string              `json:"webhook_event_type,omitempty"`
	CheckoutLink     string              `json:"checkout_link,omitempty"`
	OrderStatus      string              `json:"order_status,omitempty"`

	// Date fields (flexible parsing needed)
	NextPaymentAt   interface{} `json:"next_payment_at,omitempty"`
	NextPaymentDate interface{} `json:"next_payment_date,omitempty"`
	NextPayment     interface{} `json:"next_payment,omitempty"`
	NextBillingAt   interface{} `json:"next_billing_at,omitempty"`
	NextBillingDate interface{} `json:"next_billing_date,omitempty"`
	NextChargeAt    interface{} `json:"next_charge_at,omitempty"`
	NextChargeDate  interface{} `json:"next_charge_date,omitempty"`
	AccessUntil     interface{} `json:"access_until,omitempty"`
	AccessUntilAt   interface{} `json:"access_until_at,omitempty"`
	RefundedAt      interface{} `json:"refunded_at,omitempty"`
	ApprovedDate    interface{} `json:"approved_date,omitempty"`
	UpdatedAt       interface{} `json:"updated_at,omitempty"`
	CreatedAt       interface{} `json:"created_at,omitempty"`
}

// KiwifyOrderData represents the order data within a wrapped payload.
type KiwifyOrderData struct {
	Customer         *KiwifyCustomer     `json:"Customer,omitempty"`
	Product          *KiwifyProduct      `json:"Product,omitempty"`
	Subscription     *KiwifySubscription `json:"Subscription,omitempty"`
	WebhookEventType string              `json:"webhook_event_type,omitempty"`
	CheckoutLink     string              `json:"checkout_link,omitempty"`
	OrderStatus      string              `json:"order_status,omitempty"`

	// Date fields
	NextPaymentAt   interface{} `json:"next_payment_at,omitempty"`
	NextPaymentDate interface{} `json:"next_payment_date,omitempty"`
	NextPayment     interface{} `json:"next_payment,omitempty"`
	NextBillingAt   interface{} `json:"next_billing_at,omitempty"`
	NextBillingDate interface{} `json:"next_billing_date,omitempty"`
	NextChargeAt    interface{} `json:"next_charge_at,omitempty"`
	NextChargeDate  interface{} `json:"next_charge_date,omitempty"`
	AccessUntil     interface{} `json:"access_until,omitempty"`
	AccessUntilAt   interface{} `json:"access_until_at,omitempty"`
	RefundedAt      interface{} `json:"refunded_at,omitempty"`
	ApprovedDate    interface{} `json:"approved_date,omitempty"`
	UpdatedAt       interface{} `json:"updated_at,omitempty"`
	CreatedAt       interface{} `json:"created_at,omitempty"`
}

// KiwifyCustomer represents customer data from Kiwify webhook.
type KiwifyCustomer struct {
	Email     string `json:"email"`
	FullName  string `json:"full_name,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	Mobile    string `json:"mobile,omitempty"`
	CPF       string `json:"CPF,omitempty"`
}

// KiwifyProduct represents product data from Kiwify webhook.
type KiwifyProduct struct {
	ProductName string `json:"product_name"`
	ProductID   string `json:"product_id,omitempty"`
}

// KiwifySubscription represents subscription data from Kiwify webhook.
type KiwifySubscription struct {
	NextPaymentAt   interface{}           `json:"next_payment_at,omitempty"`
	NextPaymentDate interface{}           `json:"next_payment_date,omitempty"`
	NextPayment     interface{}           `json:"next_payment,omitempty"`
	NextBillingAt   interface{}           `json:"next_billing_at,omitempty"`
	NextBillingDate interface{}           `json:"next_billing_date,omitempty"`
	NextChargeAt    interface{}           `json:"next_charge_at,omitempty"`
	NextChargeDate  interface{}           `json:"next_charge_date,omitempty"`
	CustomerAccess  *KiwifyCustomerAccess `json:"customer_access,omitempty"`
	Charges         *KiwifyCharges        `json:"charges,omitempty"`
	StartDate       interface{}           `json:"start_date,omitempty"`
}

// KiwifyCustomerAccess represents customer access data from subscription.
type KiwifyCustomerAccess struct {
	AccessUntil interface{} `json:"access_until,omitempty"`
}

// KiwifyCharges represents charge data from subscription.
type KiwifyCharges struct {
	Future []KiwifyFutureCharge `json:"future,omitempty"`
}

// KiwifyFutureCharge represents a future charge from subscription.
type KiwifyFutureCharge struct {
	ChargeDate interface{} `json:"charge_date,omitempty"`
}

// KiwifyWebhookResult represents the response from processing a Kiwify webhook.
type KiwifyWebhookResult struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	UserID  string `json:"userId,omitempty"`
	Plan    string `json:"plan,omitempty"`
	Event   string `json:"event,omitempty"`
}

// KiwifyProcessedData represents normalized data extracted from Kiwify webhook.
type KiwifyProcessedData struct {
	Email          string
	Event          string
	ProductName    string
	ProductID      string
	FullName       string
	PhoneNumber    string
	CPF            string
	NextPaymentAt  *time.Time
	AccessUntil    *time.Time
	EventAt        *time.Time
	OrderStatus    string
	IsRefundStatus bool
}

// KiwifyEventType constants for webhook event types.
const (
	KiwifyEventOrderApproved        = "order_approved"
	KiwifyEventOrderRefunded        = "order_refunded"
	KiwifyEventSubscriptionCanceled = "subscription_canceled"
	KiwifyEventSubscriptionLate     = "subscription_late"
)

// Plan represents a subscription plan in the database.
type Plan struct {
	ID                      string   `json:"id" db:"id"`
	Slug                    string   `json:"slug" db:"slug"`
	Name                    string   `json:"name" db:"name"`
	IsActive                bool     `json:"is_active" db:"is_active"`
	KiwifyProductIDMonthly  *string  `json:"kiwify_product_id_monthly,omitempty" db:"kiwify_product_id_monthly"`
	KiwifyProductIDSemester *string  `json:"kiwify_product_id_semester,omitempty" db:"kiwify_product_id_semester"`
	KiwifyProductIDAnnual   *string  `json:"kiwify_product_id_annual,omitempty" db:"kiwify_product_id_annual"`
	CreatedAt               string   `json:"created_at" db:"created_at"`
	UpdatedAt               string   `json:"updated_at" db:"updated_at"`
	Description             *string  `json:"description,omitempty" db:"description"`
	PriceMonthly            *float64 `json:"price_monthly,omitempty" db:"price_monthly"`
	PriceSemester           *float64 `json:"price_semester,omitempty" db:"price_semester"`
	PriceAnnual             *float64 `json:"price_annual,omitempty" db:"price_annual"`
	GroupLimit              int32    `json:"group_limit" db:"group_limit"`
	StorageLimit            int64    `json:"storage_limit" db:"storage_limit"`
	Features                []string `json:"features" db:"features"`
}

// BillingSuspensionKind constants for billing suspension types.
const (
	BillingSuspensionKindPayment = "payment"
	BillingSuspensionKindManual  = "manual"
)

// ProfileBillingFields represents billing-related fields for a profile.
type ProfileBillingFields struct {
	BillingStatus           *string    `json:"billing_status,omitempty" db:"billing_status"`
	BillingSuspendedAt      *time.Time `json:"billing_suspended_at,omitempty" db:"billing_suspended_at"`
	BillingSuspensionReason *string    `json:"billing_suspension_reason,omitempty" db:"billing_suspension_reason"`
	BillingSuspensionKind   *string    `json:"billing_suspension_kind,omitempty" db:"billing_suspension_kind"`
	BillingNextPaymentAt    *time.Time `json:"billing_next_payment_at,omitempty" db:"billing_next_payment_at"`
	BillingLastEventAt      *time.Time `json:"billing_last_event_at,omitempty" db:"billing_last_event_at"`
	BillingLastEventType    *string    `json:"billing_last_event_type,omitempty" db:"billing_last_event_type"`
	PlanAccessUntil         *time.Time `json:"plan_access_until,omitempty" db:"plan_access_until"`
}
