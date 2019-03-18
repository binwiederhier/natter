#include <iostream>
#include <thread>
#include <natter.h>

using namespace std;

void broker_listen()
{
  natter_broker_listen((char*) ":10000");
}

void client_listen()
{
  natter_client_listen((char*) "bob", (char*) "localhost:10000");
}

void client_forward()
{
  natter_client_forward((char*) "alice", (char*) "localhost:10000", (char*) ":9000", (char*) "bob", (char*) ":22", 0, NULL);
}

int main()
{
  std::thread broker (broker_listen);
  std::thread alice (client_forward);
  std::thread bob (client_listen);

  broker.join();
  return 0;
}
