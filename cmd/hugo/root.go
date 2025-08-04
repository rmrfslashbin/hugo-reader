package hugo

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	rootCmd = &cobra.Command{
		Use:   "hugo-reader",
		Short: "Hugo Reader MCP Server",
		Long: `Hugo Reader is an MCP (Model-Controller-Protocol) server that provides
access to Hugo sites through their JSON endpoints.`,
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.hugo-reader.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "logging level (debug, info, warn, error)")
	rootCmd.PersistentFlags().String("server-name", "hugo-reader", "server name")
	rootCmd.PersistentFlags().String("http-timeout", "10", "HTTP timeout in seconds")
	rootCmd.PersistentFlags().String("user-agent", "HugoReader/1.0.0", "User Agent string for HTTP requests")

	// Bind flags to viper
	viper.BindPFlag("log_level", rootCmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("server_name", rootCmd.PersistentFlags().Lookup("server-name"))
	viper.BindPFlag("http_timeout", rootCmd.PersistentFlags().Lookup("http-timeout"))
	viper.BindPFlag("user_agent", rootCmd.PersistentFlags().Lookup("user-agent"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Check if HOME is set - if not, we're likely being run by an MCP client
	// and will rely entirely on environment variables for configuration
	_, homeExists := os.LookupEnv("HOME")
	
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else if homeExists {
		// Only try to load config from home if HOME is defined
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".hugo-reader" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".hugo-reader")
	}

	// Environment variables can override config file settings
	viper.SetEnvPrefix("HUGO_READER")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found and we have a HOME directory, read it in.
	if homeExists || cfgFile != "" {
		if err := viper.ReadInConfig(); err == nil {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	} else {
		fmt.Fprintln(os.Stderr, "HOME environment variable not set, relying on environment variables for configuration")
	}

	// Load environment variables
	_ = viper.BindEnv("log_level", "LOG_LEVEL")
	_ = viper.BindEnv("server_name", "MCP_SERVER_NAME") 
	_ = viper.BindEnv("http_timeout", "HUGO_READER_HTTP_TIMEOUT")
	_ = viper.BindEnv("user_agent", "HUGO_READER_USER_AGENT")
}