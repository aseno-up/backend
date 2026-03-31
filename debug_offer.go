package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Offer struct {
	ID                     string     `gorm:"primary_key;type:uuid"`
	PlanID                 string     `gorm:"type:uuid"`
	Token                  string     
	IsActive               bool       
	ValidFrom              *time.Time 
	ValidUntil             *time.Time 
	MaxUses                *int       
	CurrentUses            int        
}

func (Offer) TableName() string {
	return "offers"
}

func main() {
	dsn := "postgres://aseno:aseno_password@localhost:5433/aseno_db?sslmode=disable"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	var offer Offer
	if err := db.Where("token = ?", "MARCO50").First(&offer).Error; err != nil {
		log.Fatal("Offer not found: ", err)
	}

	fmt.Printf("Offer: %+v\n", offer)

	if !offer.IsActive {
		fmt.Printf("Offer %s is invalid: !o.IsActive\n", offer.Token)
	}
	now := time.Now()
	if offer.ValidFrom != nil {
	    fmt.Printf("ValidFrom: %s\n", offer.ValidFrom.String())
	    if now.Before(*offer.ValidFrom) {
		    fmt.Printf("Offer %s is invalid: Before ValidFrom %v\n", offer.Token, offer.ValidFrom)
	    }
	}
	if offer.ValidUntil != nil {
	    fmt.Printf("ValidUntil: %s\n", offer.ValidUntil.String())
	    if now.After(*offer.ValidUntil) {
		    fmt.Printf("Offer %s is invalid: After ValidUntil %v\n", offer.Token, offer.ValidUntil)
	    }
	}
	if offer.MaxUses != nil && offer.CurrentUses >= *offer.MaxUses {
		fmt.Printf("Offer %s is invalid: MaxUses reached %d / %d\n", offer.Token, offer.CurrentUses, *offer.MaxUses)
	}
}
