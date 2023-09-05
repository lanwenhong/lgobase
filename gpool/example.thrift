namespace go example

exception ExampleError {
    1: i32 respcd,
    2: string resperr
    3: optional string ext_json,
}

struct Myret {
    1: string ret,
}

service Example {
    i32 add(1: i32 a, 2: i32 b);
    Myret echo(1: string req)throws(1: ExampleError e);
}
