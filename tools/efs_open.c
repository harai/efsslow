#include <err.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>

int main(int argc, char *argv[]) {
  int fd;

  if (argc < 2) {
    printf("Usage: %s <file_path>\n", argv[0]);
    return 0;
  }

  while (1) {
    fd = open(argv[1], O_RDONLY);
    if (fd == -1) {
      err(EXIT_FAILURE, "Failed to open()");
    }
    if (close(fd) == -1) {
      err(EXIT_FAILURE, "Failed to close()");
    }
  }
  return 0;
}
