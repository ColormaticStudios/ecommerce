package commands

import (
	"ecommerce/config"
	"ecommerce/models"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func NewUserCmd() *cobra.Command {
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
		Long:  "Commands for managing users: set admin, create user, list users, etc.",
	}

	userCmd.AddCommand(newSetAdminCmd())
	userCmd.AddCommand(newCreateUserCmd())
	userCmd.AddCommand(newListUsersCmd())
	userCmd.AddCommand(newDeleteUserCmd())

	return userCmd
}

func newSetAdminCmd() *cobra.Command {
	var email, username string

	cmd := &cobra.Command{
		Use:   "set-admin",
		Short: "Set a user as admin",
		Long:  "Promote a user to admin role by email or username",
		Run: func(cmd *cobra.Command, args []string) {
			db := getDB()
			defer closeDB(db)

			var user models.User
			var err error

			if email != "" {
				err = db.Where("email = ?", email).First(&user).Error
			} else if username != "" {
				err = db.Where("username = ?", username).First(&user).Error
			} else {
				log.Fatal("Either --email or --username must be provided")
			}

			if err != nil {
				if err == gorm.ErrRecordNotFound {
					log.Fatalf("User not found")
				}
				log.Fatalf("Error finding user: %v", err)
			}

			user.Role = "admin"
			if err := db.Save(&user).Error; err != nil {
				log.Fatalf("Error updating user: %v", err)
			}

			fmt.Printf("✓ User '%s' (%s) has been set as admin\n", user.Username, user.Email)
		},
	}

	cmd.Flags().StringVarP(&email, "email", "e", "", "User email address")
	cmd.Flags().StringVarP(&username, "username", "u", "", "User username")
	cmd.MarkFlagsOneRequired("email", "username")

	return cmd
}

func newCreateUserCmd() *cobra.Command {
	var email, username, password, name, role string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new user",
		Long:  "Create a new user account with email, username, and password",
		Run: func(cmd *cobra.Command, args []string) {
			db := getDB()
			defer closeDB(db)

			// Validate required fields
			if email == "" || username == "" || password == "" {
				log.Fatal("Email, username, and password are required")
			}

			// Check if user already exists
			var existingUser models.User
			if err := db.Where("email = ? OR username = ?", email, username).First(&existingUser).Error; err == nil {
				log.Fatalf("User with email '%s' or username '%s' already exists", email, username)
			}

			// Hash password
			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				log.Fatalf("Error hashing password: %v", err)
			}

			// Generate subject ID
			subject := generateSubjectID(email)

			// Set default role
			if role == "" {
				role = "customer"
			}
			if role != "admin" && role != "customer" {
				log.Fatal("Role must be either 'admin' or 'customer'")
			}

			// Create user
			user := models.User{
				Subject:      subject,
				Username:     username,
				Email:        email,
				PasswordHash: string(hashedPassword),
				Name:         name,
				Role:         role,
				Currency:     "USD",
			}

			if err := db.Create(&user).Error; err != nil {
				log.Fatalf("Error creating user: %v", err)
			}

			fmt.Printf("✓ User created successfully:\n")
			fmt.Printf("  ID: %d\n", user.ID)
			fmt.Printf("  Username: %s\n", user.Username)
			fmt.Printf("  Email: %s\n", user.Email)
			fmt.Printf("  Role: %s\n", user.Role)
		},
	}

	cmd.Flags().StringVarP(&email, "email", "e", "", "User email address (required)")
	cmd.Flags().StringVarP(&username, "username", "u", "", "User username (required)")
	cmd.Flags().StringVarP(&password, "password", "p", "", "User password (required)")
	cmd.Flags().StringVarP(&name, "name", "n", "", "User full name")
	cmd.Flags().StringVarP(&role, "role", "r", "customer", "User role (admin or customer)")
	cmd.MarkFlagRequired("email")
	cmd.MarkFlagRequired("username")
	cmd.MarkFlagRequired("password")

	return cmd
}

func newListUsersCmd() *cobra.Command {
	var role string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all users",
		Long:  "List all users in the database, optionally filtered by role",
		Run: func(cmd *cobra.Command, args []string) {
			db := getDB()
			defer closeDB(db)

			var users []models.User
			query := db.Model(&models.User{})

			if role != "" {
				query = query.Where("role = ?", role)
			}

			if err := query.Find(&users).Error; err != nil {
				log.Fatalf("Error listing users: %v", err)
			}

			if len(users) == 0 {
				fmt.Println("No users found")
				return
			}

			fmt.Printf("Found %d user(s):\n\n", len(users))
			fmt.Printf("%-5s %-20s %-30s %-10s %-20s\n", "ID", "Username", "Email", "Role", "Name")
			fmt.Println("--------------------------------------------------------------------------------")
			for _, user := range users {
				fmt.Printf("%-5d %-20s %-30s %-10s %-20s\n",
					user.ID, user.Username, user.Email, user.Role, user.Name)
			}
		},
	}

	cmd.Flags().StringVarP(&role, "role", "r", "", "Filter by role (admin or customer)")

	return cmd
}

func newDeleteUserCmd() *cobra.Command {
	var email, username string
	var confirm bool

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user",
		Long:  "Delete a user account by email or username",
		Run: func(cmd *cobra.Command, args []string) {
			db := getDB()
			defer closeDB(db)

			var user models.User
			var err error

			if email != "" {
				err = db.Where("email = ?", email).First(&user).Error
			} else if username != "" {
				err = db.Where("username = ?", username).First(&user).Error
			} else {
				log.Fatal("Either --email or --username must be provided")
			}

			if err != nil {
				if err == gorm.ErrRecordNotFound {
					log.Fatalf("User not found")
				}
				log.Fatalf("Error finding user: %v", err)
			}

			if !confirm {
				fmt.Printf("Are you sure you want to delete user '%s' (%s)? (yes/no): ", user.Username, user.Email)
				var response string
				fmt.Scanln(&response)
				if response != "yes" && response != "y" {
					fmt.Println("Cancelled")
					return
				}
			}

			if err := db.Delete(&user).Error; err != nil {
				log.Fatalf("Error deleting user: %v", err)
			}

			fmt.Printf("✓ User '%s' (%s) has been deleted\n", user.Username, user.Email)
		},
	}

	cmd.Flags().StringVarP(&email, "email", "e", "", "User email address")
	cmd.Flags().StringVarP(&username, "username", "u", "", "User username")
	cmd.Flags().BoolVarP(&confirm, "yes", "y", false, "Skip confirmation prompt")
	cmd.MarkFlagsOneRequired("email", "username")

	return cmd
}

// Helper functions

func getDB() *gorm.DB {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := gorm.Open(postgres.Open(cfg.DBURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		},
	)
	db = db.Session(&gorm.Session{Logger: gormLogger})

	if err := db.AutoMigrate(
		&models.User{},
		&models.Product{},
		&models.Order{},
		&models.OrderItem{},
		&models.Cart{},
		&models.CartItem{},
		&models.MediaObject{},
		&models.MediaVariant{},
		&models.MediaReference{},
	); err != nil {
		log.Fatalf("Failed to migrate database schema: %v", err)
	}

	return db
}

func closeDB(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		return
	}
	sqlDB.Close()
}

func generateSubjectID(email string) string {
	hash, _ := bcrypt.GenerateFromPassword([]byte(email), bcrypt.DefaultCost)
	subject := ""
	for _, b := range hash[:16] {
		subject += string(rune(97 + (int(b) % 26)))
	}
	return subject
}
