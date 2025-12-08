namespace go server


struct GetUserRequest {
    1: required i32 user_id,     
    2: required string name
}

service ServerTest {
    i32 add(1:i32 a, 2:i32 b);
    i32 add1(1:i16 magic, 2:i16 ver, 3:map<string, string> ext, 4:i32 a, 5:i32 b);
    i32 postUser(1: GetUserRequest req);
}
