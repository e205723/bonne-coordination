package api

import (
    "crypto/rand"
    "crypto/sha256"
    "database/sql"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "log"
    "math/big"
    "net/http"
    "time"
    "strings"
    "github.com/dgrijalva/jwt-go"
    _ "github.com/lib/pq"
)

type Server struct {
    Db *sql.DB
}

type SignUpRequest struct {
    Name                   string `json:"name"`
    Password               string `json:"password"`
    PasswordConfirmination string `json:"passwordConfirmination"`
}

type SignInRequest struct {
    Name                   string `json:"name"`
    Password               string `json:"password"`
}

type SkeletalTypeRequest struct {
    BodyImpression string `json:"bodyImpression"`
    FingerJointSize string `json:"fingerJointSize"`
    WristShape string `json:"wristShape"`
    WristAnkcle string `json:"wristAnkcle"`
    ClavicleImpression string `json:"clavicleImpression"`
    KneecapImpression string `json:"kneecapImpression"`
    UnsuitableClothe string `json:"unsuitableClothe"`
}

type SkeletalTypeResponse struct {
    SkeletalType string `json:"skeletalType"`
}

type SignUpResponse struct {
    Name string `json:"name"`
}

type SignInResponse struct {
    Name string `json:"name"`
}

type Claims struct {
    Name string
    jwt.StandardClaims
}

var charset62 = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

var jwtKey = []byte(RandomString(511))

func RandomString(length int) string {
    randomString := make([]rune, length)
    for i := range randomString {
        randomNumber, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset62))))
        if err != nil {
            log.Println(err)
            return ""
        }
        randomString[i] = charset62[int(randomNumber.Int64())]
    }
    return string(randomString)
}

func SetJwtInCookie(w http.ResponseWriter, userName string) {
    expirationTime := time.Now().Add(672 * time.Hour)
    claims := &Claims{
        Name: userName,
        StandardClaims: jwt.StandardClaims{
            ExpiresAt: expirationTime.Unix(),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    tokenString, err := token.SignedString(jwtKey)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        return
    }
    cookie := &http.Cookie{
        Name:    "token",
        Value:   tokenString,
        Expires: expirationTime,
    }
    http.SetCookie(w, cookie)
}

func LoadClaimsFromJwt (w http.ResponseWriter, r *http.Request) (*Claims) {
    c, err := r.Cookie("token")
    if err != nil {
        if err == http.ErrNoCookie {
            w.WriteHeader(http.StatusUnauthorized)
            return &Claims{}
        }
        w.WriteHeader(http.StatusBadRequest)
        return &Claims{}
    }

    tknStr := c.Value
    claims := &Claims{}
    tkn, err := jwt.ParseWithClaims(tknStr, claims, func(token *jwt.Token) (interface{}, error) {
        return jwtKey, nil
    })
    if err != nil {
        if err == jwt.ErrSignatureInvalid {
            w.WriteHeader(http.StatusUnauthorized)
            return &Claims{}
        }
        w.WriteHeader(http.StatusBadRequest)
        return &Claims{}
    }
    if !tkn.Valid {
        w.WriteHeader(http.StatusUnauthorized)
        return &Claims{}
    }
    return claims
}

func (s *Server) SignUp(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    var signUpRequest SignUpRequest
    decoder := json.NewDecoder(r.Body)
    decodeError := decoder.Decode(&signUpRequest)
    if decodeError != nil {
        log.Println("[ERROR]", decodeError)
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    if signUpRequest.Password == "" {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    if signUpRequest.Password != signUpRequest.PasswordConfirmination {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    passwordHash32Byte := sha256.Sum256([]byte(signUpRequest.Password))
    passwordHashURLSafe := base64.URLEncoding.EncodeToString(passwordHash32Byte[:])
    queryToReGisterUser := fmt.Sprintf("INSERT INTO users (name, password_hash) VALUES ('%s', '%s')", signUpRequest.Name, passwordHashURLSafe)
    _, queryRrror := s.Db.Exec(queryToReGisterUser)
    if queryRrror != nil {
        log.Println("[ERROR]", queryRrror)
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    SetJwtInCookie(w, signUpRequest.Name)
    w.Header().Set("Content-Type", "application/json")
    response := SignUpResponse{
        Name: signUpRequest.Name,
    }
    jsonResponse, err := json.Marshal(response)
    if err != nil {
        log.Fatalf("Error happened in JSON marshal. Err: %s", err)
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    w.Write(jsonResponse)
}

func (s *Server) SignIn(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    var signInRequest SignInRequest
    decoder := json.NewDecoder(r.Body)
    decodeError := decoder.Decode(&signInRequest)
    if decodeError != nil {
        log.Println("[ERROR]", decodeError)
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    passwordHash32Byte := sha256.Sum256([]byte(signInRequest.Password))
    passwordHashURLSafe := base64.URLEncoding.EncodeToString(passwordHash32Byte[:])
    query := fmt.Sprintf("SELECT password_hash FROM users WHERE name = '%s'", signInRequest.Name)
    var correctPasswordHashURLSafe string
    if queryError := s.Db.QueryRow(query).Scan(&correctPasswordHashURLSafe); queryError != nil {
        log.Println("[ERROR]", queryError)
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    correctPasswordHashURLSafe = strings.TrimRight(correctPasswordHashURLSafe, " ")
    if passwordHashURLSafe != correctPasswordHashURLSafe {
        w.WriteHeader(http.StatusUnauthorized)
        return
    }
    SetJwtInCookie(w, signInRequest.Name)
    w.Header().Set("Content-Type", "application/json")
    response := SignInRequest{
        Name: signInRequest.Name,
    }
    jsonResponse, err := json.Marshal(response)
    if err != nil {
        log.Fatalf("Error happened in JSON marshal. Err: %s", err)
    }
    w.Write(jsonResponse)
}

func (s *Server) SignInWithJwt(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    claims := LoadClaimsFromJwt(w, r)
    w.Header().Set("Content-Type", "application/json")
    response := SignInResponse{
        Name: claims.Name,
    }
    jsonResponse, err := json.Marshal(response)
    if err != nil {
        log.Fatalf("Error happened in JSON marshal. Err: %s", err)
    }
    w.Write(jsonResponse)
}

func (s *Server) GetSkeletalType(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    var skeletalTypeRequest SkeletalTypeRequest
    decoder := json.NewDecoder(r.Body)
    decodeError := decoder.Decode(&skeletalTypeRequest)
    if decodeError != nil {
        log.Println("[ERROR]", decodeError)
        w.WriteHeader(http.StatusBadRequest)
        return
    }
    straightScore := 0
    waveScore := 0
    naturalScore := 0
    if skeletalTypeRequest.BodyImpression == "???????????????????????????" {
        straightScore += 1
    } else if skeletalTypeRequest.BodyImpression == "?????????????????????????????????????????????" {
        waveScore += 1
    } else if skeletalTypeRequest.BodyImpression == "?????????????????????" {
        naturalScore += 1
    }
    if skeletalTypeRequest.FingerJointSize == "?????????" {
        straightScore += 1
    } else if skeletalTypeRequest.FingerJointSize == "?????????" {
        waveScore += 1
    } else if skeletalTypeRequest.FingerJointSize == "?????????" {
        naturalScore += 1
    }
    if skeletalTypeRequest.WristShape == "?????????????????????????????????" {
        straightScore += 1
    } else if skeletalTypeRequest.WristShape == "???????????????????????????????????????????????????????????????" {
        waveScore += 1
    } else if skeletalTypeRequest.WristShape == "??????????????????????????????????????????" {
        naturalScore += 1
    }
    if skeletalTypeRequest.WristAnkcle == "??????????????????????????????????????????" {
        straightScore += 1
    } else if skeletalTypeRequest.WristAnkcle == "???????????????????????????????????????" {
        waveScore += 1
    } else if skeletalTypeRequest.WristAnkcle == "??????????????????????????????????????????????????????" {
        naturalScore += 1
    }
    if skeletalTypeRequest.ClavicleImpression == "??????????????????????????????????????????" {
        straightScore += 1
    } else if skeletalTypeRequest.ClavicleImpression == "??????????????????" {
        waveScore += 1
    } else if skeletalTypeRequest.ClavicleImpression == "?????????????????????????????????" {
        naturalScore += 1
    }
    if skeletalTypeRequest.KneecapImpression == "??????????????????????????????????????????" {
        straightScore += 1
    } else if skeletalTypeRequest.KneecapImpression == "??????????????????????????????????????????????????????" {
        waveScore += 1
    } else if skeletalTypeRequest.KneecapImpression == "?????????" {
        naturalScore += 1
    }
    if skeletalTypeRequest.UnsuitableClothe == "?????????????????????" {
        straightScore += 1
    } else if skeletalTypeRequest.UnsuitableClothe == "??????????????????" {
        waveScore += 1
    } else if skeletalTypeRequest.UnsuitableClothe == "?????????????????????T?????????" {
        naturalScore += 1
    }
    var response SkeletalTypeResponse
    if straightScore > waveScore && straightScore > naturalScore {
        response = SkeletalTypeResponse{SkeletalType: "???????????????"}
    } else if waveScore > straightScore && waveScore > naturalScore {
        response = SkeletalTypeResponse{SkeletalType: "????????????"}
    } else {
        response = SkeletalTypeResponse{SkeletalType: "???????????????"}
    }
    responseJson, err := json.Marshal(response)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(responseJson)
}
