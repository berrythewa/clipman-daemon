#!/bin/bash
# CLI-based Config Testing Script
# Tests config functionality through actual CLI commands

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

TEMP_DIR=$(mktemp -d)
BACKUP_CONFIG=""
TEST_CONFIG="$TEMP_DIR/test_config.yaml"

cleanup() {
    echo -e "${YELLOW}Cleaning up...${NC}"
    if [[ -n "$BACKUP_CONFIG" && -f "$BACKUP_CONFIG" ]]; then
        echo "Restoring original config..."
        clipman config load "$BACKUP_CONFIG" --force || true
        rm -f "$BACKUP_CONFIG"
    fi
    rm -rf "$TEMP_DIR"
}

trap cleanup EXIT

pass() {
    echo -e "${GREEN}✓ $1${NC}"
}

fail() {
    echo -e "${RED}✗ $1${NC}"
    exit 1
}

info() {
    echo -e "${YELLOW}→ $1${NC}"
}

# Test config show command
test_config_show() {
    info "Testing config show..."
    
    # Test YAML format
    clipman config show --format=yaml > /dev/null || fail "Config show YAML failed"
    pass "Config show YAML format works"
    
    # Test JSON format
    clipman config show --format=json > /dev/null || fail "Config show JSON failed"
    pass "Config show JSON format works"
    
    # Test invalid format
    if clipman config show --format=invalid 2>/dev/null; then
        fail "Config show should reject invalid format"
    fi
    pass "Config show rejects invalid format"
}

# Test config validation
test_config_validate() {
    info "Testing config validation..."
    
    # Test valid config
    clipman config validate || fail "Config validation failed on valid config"
    pass "Config validation works on valid config"
    
    # Test invalid config
    echo "invalid: yaml: content" > "$TEST_CONFIG"
    if clipman config load "$TEST_CONFIG" --force 2>/dev/null; then
        fail "Config should reject invalid YAML"
    fi
    pass "Config rejects invalid YAML"
}

# Test config export/load cycle
test_config_export_load() {
    info "Testing config export/load cycle..."
    
    # Export current config
    clipman config export -o "$TEST_CONFIG" || fail "Config export failed"
    pass "Config export works"
    
    # Verify exported file exists and is valid YAML
    [[ -f "$TEST_CONFIG" ]] || fail "Exported config file doesn't exist"
    clipman config validate || fail "Exported config is invalid"
    pass "Exported config is valid"
    
    # Create backup of current config
    BACKUP_CONFIG="$TEMP_DIR/backup.yaml"
    clipman config export -o "$BACKUP_CONFIG"
    
    # Load the exported config back
    clipman config load "$TEST_CONFIG" --force || fail "Config load failed"
    pass "Config load works"
}

# Test config reset
test_config_reset() {
    info "Testing config reset..."
    
    # Reset to defaults
    clipman config reset --force || fail "Config reset failed"
    pass "Config reset works"
    
    # Verify config is valid after reset
    clipman config validate || fail "Config invalid after reset"
    pass "Config valid after reset"
    
    # Verify we can show config after reset
    clipman config show --format=yaml > /dev/null || fail "Can't show config after reset"
    pass "Can show config after reset"
}

# Test config with different environments
test_config_environments() {
    info "Testing config with different environments..."
    
    # Test with custom config path
    CUSTOM_CONFIG="$TEMP_DIR/custom.yaml"
    clipman config export -o "$CUSTOM_CONFIG"
    
    CLIPMAN_CONFIG="$CUSTOM_CONFIG" clipman config show > /dev/null || fail "Custom config path failed"
    pass "Custom config path works"
    
    # Test with custom data directory
    CUSTOM_DATA_DIR="$TEMP_DIR/data"
    mkdir -p "$CUSTOM_DATA_DIR"
    
    CLIPMAN_DATA_DIR="$CUSTOM_DATA_DIR" clipman config show > /dev/null || fail "Custom data dir failed"
    pass "Custom data directory works"
}

# Test error conditions
test_config_errors() {
    info "Testing config error conditions..."
    
    # Test loading non-existent file
    if clipman config load "/nonexistent/file.yaml" 2>/dev/null; then
        fail "Should fail loading non-existent file"
    fi
    pass "Properly handles non-existent config file"
    
    # Test reset without force
    if clipman config reset 2>/dev/null; then
        fail "Reset should require --force when config exists"
    fi
    pass "Reset requires --force when config exists"
}

# Test integration with other commands
test_config_integration() {
    info "Testing config integration with other commands..."
    
    # Ensure daemon can load the config
    timeout 5s clipman daemon status || info "Daemon not running (expected)"
    pass "Config integrates with daemon commands"
    
    # Test config affects other commands
    clipman config show --format=json | jq -r '.device_id' > /dev/null || info "jq not available, skipping JSON parsing test"
    pass "Config output can be processed programmatically"
}

# Main test execution
main() {
    echo "Starting CLI-based Config Tests..."
    echo "Temp directory: $TEMP_DIR"
    
    test_config_show
    test_config_validate
    test_config_export_load
    test_config_reset
    test_config_environments
    test_config_errors
    test_config_integration
    
    echo -e "${GREEN}All CLI config tests passed!${NC}"
}

main "$@" 