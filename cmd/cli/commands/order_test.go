package commands

import (
	"testing"
	"time"

	"ecommerce/models"
)

func TestBuildOrderInspectResponseIncludesOperationalData(t *testing.T) {
	db := newTestDB(t,
		&models.User{},
		&models.CheckoutSession{},
		&models.Brand{},
		&models.Product{},
		&models.ProductOption{},
		&models.ProductOptionValue{},
		&models.ProductVariant{},
		&models.ProductVariantOptionValue{},
		&models.ProductAttribute{},
		&models.ProductAttributeValue{},
		&models.SEOMetadata{},
		&models.Order{},
		&models.OrderItem{},
		&models.PaymentIntent{},
		&models.PaymentTransaction{},
		&models.OrderCheckoutSnapshot{},
		&models.OrderCheckoutSnapshotItem{},
		&models.OrderTaxLine{},
		&models.OrderStatusHistory{},
		&models.Shipment{},
		&models.ShipmentRate{},
		&models.ShipmentPackage{},
		&models.TrackingEvent{},
	)

	user := models.User{Subject: "sub-1", Username: "ada", Email: "ada@example.com"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	session := models.CheckoutSession{
		PublicToken: "token",
		UserID:      &user.ID,
		Status:      models.CheckoutSessionStatusConverted,
		ExpiresAt:   time.Now().Add(time.Hour),
		LastSeenAt:  time.Now(),
	}
	if err := db.Create(&session).Error; err != nil {
		t.Fatalf("create checkout session: %v", err)
	}

	product := models.Product{
		SKU:         "PROD-1",
		Name:        "Main",
		Description: "Main product",
		Price:       models.MoneyFromFloat(19.99),
		Stock:       5,
	}
	if err := db.Create(&product).Error; err != nil {
		t.Fatalf("create product: %v", err)
	}

	variant := models.ProductVariant{
		ProductID:   product.ID,
		SKU:         "PROD-1-M",
		Title:       "Main / M",
		Price:       models.MoneyFromFloat(19.99),
		Stock:       5,
		Position:    1,
		IsPublished: true,
	}
	if err := db.Select("*").Create(&variant).Error; err != nil {
		t.Fatalf("create variant: %v", err)
	}
	if err := db.Model(&product).Update("default_variant_id", variant.ID).Error; err != nil {
		t.Fatalf("set default variant: %v", err)
	}

	order := models.Order{
		UserID:            &user.ID,
		CheckoutSessionID: session.ID,
		Total:             models.MoneyFromFloat(29.99),
		Status:            models.StatusPaid,
	}
	if err := db.Create(&order).Error; err != nil {
		t.Fatalf("create order: %v", err)
	}

	orderItem := models.OrderItem{
		OrderID:          order.ID,
		ProductVariantID: variant.ID,
		VariantSKU:       variant.SKU,
		VariantTitle:     variant.Title,
		Quantity:         1,
		Price:            models.MoneyFromFloat(19.99),
	}
	if err := db.Create(&orderItem).Error; err != nil {
		t.Fatalf("create order item: %v", err)
	}

	snapshot := models.OrderCheckoutSnapshot{
		CheckoutSessionID:  session.ID,
		OrderID:            &order.ID,
		Currency:           "USD",
		Subtotal:           models.MoneyFromFloat(19.99),
		ShippingAmount:     models.MoneyFromFloat(5),
		TaxAmount:          models.MoneyFromFloat(5),
		Total:              models.MoneyFromFloat(29.99),
		PaymentProviderID:  "stripe",
		ShippingProviderID: "shippo",
		PaymentDataJSON:    `{"intent":"pi_123"}`,
		ShippingDataJSON:   `{"rate":"ground"}`,
		TaxDataJSON:        `{"mode":"finalized"}`,
		ExpiresAt:          time.Now().Add(time.Hour),
	}
	if err := db.Create(&snapshot).Error; err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	snapshotItem := models.OrderCheckoutSnapshotItem{
		SnapshotID:       snapshot.ID,
		ProductVariantID: variant.ID,
		VariantSKU:       variant.SKU,
		VariantTitle:     variant.Title,
		Quantity:         1,
		Price:            models.MoneyFromFloat(19.99),
	}
	if err := db.Create(&snapshotItem).Error; err != nil {
		t.Fatalf("create snapshot item: %v", err)
	}
	if err := db.Create(&models.OrderTaxLine{
		OrderID:            order.ID,
		SnapshotID:         snapshot.ID,
		SnapshotItemID:     &snapshotItem.ID,
		LineType:           models.TaxLineTypeItem,
		TaxProviderID:      "avalara",
		ProductVariantID:   &variant.ID,
		TaxableAmount:      models.MoneyFromFloat(19.99),
		TaxAmount:          models.MoneyFromFloat(5),
		TaxRateBasisPoints: 2500,
		FinalizedAt:        time.Now(),
	}).Error; err != nil {
		t.Fatalf("create tax line: %v", err)
	}

	if err := db.Create(&models.PaymentIntent{
		OrderID:          order.ID,
		SnapshotID:       snapshot.ID,
		Provider:         "stripe",
		Status:           models.PaymentIntentStatusCaptured,
		AuthorizedAmount: models.MoneyFromFloat(29.99),
		CapturedAmount:   models.MoneyFromFloat(29.99),
		Currency:         "USD",
		Version:          1,
	}).Error; err != nil {
		t.Fatalf("create payment intent: %v", err)
	}

	var intent models.PaymentIntent
	if err := db.First(&intent).Error; err != nil {
		t.Fatalf("reload payment intent: %v", err)
	}
	if err := db.Create(&models.PaymentTransaction{
		PaymentIntentID:     intent.ID,
		Operation:           models.PaymentTransactionOperationCapture,
		ProviderTxnID:       "txn_123",
		IdempotencyKey:      "capture-1",
		Amount:              models.MoneyFromFloat(29.99),
		Status:              models.PaymentTransactionStatusSucceeded,
		RawResponseRedacted: `{"ok":true}`,
	}).Error; err != nil {
		t.Fatalf("create payment transaction: %v", err)
	}

	if err := db.Create(&models.OrderStatusHistory{
		OrderID:       order.ID,
		FromStatus:    models.StatusPending,
		ToStatus:      models.StatusPaid,
		Reason:        "payment_captured",
		Source:        "admin",
		Actor:         "tester",
		CorrelationID: "corr-1",
	}).Error; err != nil {
		t.Fatalf("create status history: %v", err)
	}

	rate := models.ShipmentRate{
		OrderID:        order.ID,
		SnapshotID:     snapshot.ID,
		Provider:       "shippo",
		ProviderRateID: "rate_1",
		ServiceCode:    "ground",
		ServiceName:    "Ground",
		Amount:         models.MoneyFromFloat(5),
		Currency:       "USD",
		Selected:       true,
	}
	if err := db.Create(&rate).Error; err != nil {
		t.Fatalf("create shipment rate: %v", err)
	}

	shipment := models.Shipment{
		OrderID:            order.ID,
		SnapshotID:         snapshot.ID,
		Provider:           "shippo",
		ShipmentRateID:     rate.ID,
		ProviderShipmentID: "shp_1",
		Status:             models.ShipmentStatusLabelPurchased,
		Currency:           "USD",
		ServiceCode:        "ground",
		ServiceName:        "Ground",
		Amount:             models.MoneyFromFloat(5),
	}
	if err := db.Create(&shipment).Error; err != nil {
		t.Fatalf("create shipment: %v", err)
	}
	if err := db.Model(&rate).Update("shipment_id", shipment.ID).Error; err != nil {
		t.Fatalf("link shipment rate: %v", err)
	}
	if err := db.Create(&models.ShipmentPackage{
		ShipmentID:  shipment.ID,
		Reference:   "pkg-1",
		WeightGrams: 500,
	}).Error; err != nil {
		t.Fatalf("create shipment package: %v", err)
	}
	if err := db.Create(&models.TrackingEvent{
		ShipmentID:      shipment.ID,
		Provider:        "shippo",
		ProviderEventID: "evt_1",
		Status:          "IN_TRANSIT",
		TrackingNumber:  "TRACK123",
		OccurredAt:      time.Now(),
	}).Error; err != nil {
		t.Fatalf("create tracking event: %v", err)
	}

	response, err := buildOrderInspectResponse(db, nil, order.ID)
	if err != nil {
		t.Fatalf("build order inspect response: %v", err)
	}

	if response.User == nil || response.User.ID != user.ID {
		t.Fatalf("expected user %d, got %+v", user.ID, response.User)
	}
	if response.CheckoutSession == nil || response.CheckoutSession.ID != session.ID {
		t.Fatalf("expected checkout session %d, got %+v", session.ID, response.CheckoutSession)
	}
	if len(response.Payments.Intents) != 1 {
		t.Fatalf("expected one payment intent, got %+v", response.Payments)
	}
	if len(response.StatusHistory) != 1 {
		t.Fatalf("expected one status history row, got %+v", response.StatusHistory)
	}
	if len(response.Snapshots) != 1 || response.Snapshots[0].PaymentData == nil {
		t.Fatalf("expected decoded snapshot payment data, got %+v", response.Snapshots)
	}
	if len(response.Shipments) != 1 || len(response.Shipments[0].TrackingEvents) != 1 {
		t.Fatalf("expected shipment with tracking event, got %+v", response.Shipments)
	}
}
