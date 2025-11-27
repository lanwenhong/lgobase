namespace go server


struct GetUserRequest {
    1: required i32 user_id,     
    2: required string name
}

service ServerTest {
    i32 add(1:i32 a, 2:i32 b);
    i32 postUser(1: GetUserRequest req);
}
