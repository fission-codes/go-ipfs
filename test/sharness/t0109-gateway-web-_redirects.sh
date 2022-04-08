#!/usr/bin/env bash

test_description="Test HTTP Gateway _redirects support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

## ============================================================================
## Test _redirects file support
## ============================================================================

# Directory tree crafted to test _redirects file support
test_expect_success "Add the _redirects file test directory" '
  mkdir -p testredirect/ &&
  echo "index.html" > testredirect/index.html &&
  echo "one.html" > testredirect/one.html &&
  echo "two.html" > testredirect/two.html &&
  echo "^/redirect-one$ /one.html" > testredirect/_redirects &&
  echo "^/301-redirect-one$ /one.html 301" >> testredirect/_redirects &&
  echo "^/302-redirect-two$ /two.html 302" >> testredirect/_redirects &&
  echo "^/200-index$ /index.html 200" >> testredirect/_redirects &&
  echo "^/*$ /index.html 200" >> testredirect/_redirects &&
  REDIRECTS_DIR_CID=$(ipfs add -Qr --cid-version 1 testredirect)
'

REDIRECTS_DIR_HOSTNAME="${REDIRECTS_DIR_CID}.ipfs.localhost:$GWAY_PORT"

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/redirect-one redirects with default of 302, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "302 Found" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/301-redirect-one redirects with 301, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/301-redirect-one" > response &&
  test_should_contain "one.html" response &&
  test_should_contain "301 Moved Permanently" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/302-redirect-two redirects with 302, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/302-redirect-two" > response &&
  test_should_contain "two.html" response &&
  test_should_contain "302 Found" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/200-index returns 200, per _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/200-index" > response &&
  test_should_contain "index.html" response &&
  test_should_contain "200 OK" response
'

test_expect_success "request for $REDIRECTS_DIR_HOSTNAME/has-no-redirects-entry returns 404, since not in _redirects file" '
  curl -sD - --resolve $REDIRECTS_DIR_HOSTNAME:127.0.0.1 "http://$REDIRECTS_DIR_HOSTNAME/has-no-redirects-entry" > response &&
  test_should_contain "404 Not Found" response
'

test_expect_success "request for http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one returns 404, no _redirects since no origin isolation" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$REDIRECTS_DIR_CID/301-redirect-one" > response &&
  test_should_contain "404 Not Found" response
'