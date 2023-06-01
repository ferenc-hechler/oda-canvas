/*
 * Private Vault - OpenAPI 3.0
 *
 * This is a simple key-value store  Some useful links: - [The sources](https://github.com/.../openapi.yaml)
 *
 * API version: 0.1.0
 * Contact: ferenc.hechler@gmail.com
 * Generated by: Swagger Codegen (https://github.com/swagger-api/swagger-codegen.git)
 */
package main

import (
	"net/http"
	"fmt"
	"io"
	"log"
	"os"
    "regexp"
    "encoding/json"
	vault "github.com/hashicorp/vault/api"
	"context"
)


var config *vault.Config
var client *vault.Client

func getEnvVar(envVarName string, defaultValue string) string {
	result := os.Getenv(envVarName)
	if result == "" {
		result = defaultValue
	}
	return result
}

func tok(path string) (string) {
    b, err := os.ReadFile(path) 
    if err != nil {
        fmt.Print(err)
    	log.Fatalf("unable to read token: %v", err)
    }
    token := string(b)
	return token
}
func jwt_login(jwt_file string) (string, error) {
	// vault write auth/jwtk8s/login role=comp-123-role jwt=@/var/run/secrets/kubernetes.io/serviceaccount/token
	log.Println("login jwt")
	jwt := tok(jwt_file)
	log.Println(jwt)
	// set auth info jwt and role
	options := map[string]interface{}{
		"jwt": jwt,
		"role": "comp-123-role",
	}
	log.Println(options)
	path := fmt.Sprintf("auth/%v/login", "jwtk8s")
	// PUT call to get a token
	auth_secret, err := client.Logical().Write(path, options)
	if err != nil {
		return "", err 
	}
	token := auth_secret.Auth.ClientToken
	return token, nil
}

func init_vault() {
	v_addr := getEnvVar("VAULT_ADDR", "https://vault.k8s.feri.ai")
    log.Println("init vault ", v_addr)
	config = vault.DefaultConfig()
	config.Address = v_addr
	var err error
	client, err = vault.NewClient(config)
	if err != nil {
    	log.Fatalf("init vault %v failed: %v", v_addr, err)
	}
	
	jwt_file := getEnvVar("JWT_FILE", "/var/run/secrets/kubernetes.io/serviceaccount/token") // jwt_file = "jwt-encrypted.txt"
    log.Println("jwt auth from ", jwt_file)
	token, err := jwt_login(jwt_file)
	if err != nil {
    	log.Fatalf("init vault %v failed: %v", v_addr, err)
	}
	
	client.SetToken(token)
}

// https://www.informit.com/articles/article.aspx?p=2861456&seqNum=7

func extractPath(text string, pattern string) string {
	return regexp.MustCompile(pattern).ReplaceAllString(text, "$1")
}

func extractKey(r *http.Request) string {
	return extractPath(r.URL.Path, "/.*/secret/([^/]+)")
}

func extractSecret(r *http.Request) Secret {
    sec := Secret{}
    reqBody, err := io.ReadAll(r.Body)
    if err != nil {
        return sec
    }
    err = json.Unmarshal(reqBody, &sec)
    return sec
}


func CreateOrUpdateSecret(w http.ResponseWriter, r *http.Request) {
    sec := extractSecret(r)
    fmt.Printf("(KEY=%s, VAL=%s)\n", sec.Key, sec.Value)
    comppath := fmt.Sprintf("component/123/%v", sec.Key)
   	secretData := map[string]interface{}{
		"value": sec.Value,
	}
	ctx := context.Background()
	var err error
	_, err = client.KVv2("comp-secrets").Put(ctx, comppath, secretData)
	if err != nil {
		fmt.Printf("error: %v\n", err)
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("ERROR 403: forbidden"))
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
}

func DeleteSecret(w http.ResponseWriter, r *http.Request) {
	key := extractKey(r)
    fmt.Printf("KEY: '%s'\n", key)
    comppath := fmt.Sprintf("component/123/%v", key)
    ctx := context.Background()
    err := client.KVv2("comp-secrets").Delete(ctx, comppath)
    if err != nil {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ERROR 403: forbidden"))
		return
    }
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
}

func GetSecretByKey(w http.ResponseWriter, r *http.Request) {
	key := extractKey(r)
    fmt.Printf("KEY: '%s'\n", key)
    ctx := context.Background()
    log.Println("client get")
    comppath := fmt.Sprintf("component/123/%v", key)
    secret, err2 := client.KVv2("comp-secrets").Get(ctx, comppath)
	if err2 != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("ERROR 404: key not found"))
		return
	}
	value, ok := secret.Data["value"].(string)
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("ERROR 404: missing value"))
		return
	}	
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	sec := Secret{key, value}
	jsonResp, _ := json.Marshal(sec)
	w.Write(jsonResp)
}

func CreateSecret(w http.ResponseWriter, r *http.Request) {
	CreateOrUpdateSecret(w, r)
}

func UpdateSecret(w http.ResponseWriter, r *http.Request) {
	CreateOrUpdateSecret(w, r)
}
