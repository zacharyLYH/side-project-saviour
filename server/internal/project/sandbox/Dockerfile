# The shared project sandbox: everything the platform itself needs to
# function (clone, tmux sessions, port detection, private repos), nothing
# more — users install what they want inside their running sandbox.
FROM ubuntu:24.04

RUN apt-get update \
	&& apt-get install -y --no-install-recommends git tmux iproute2 openssh-client ca-certificates \
	&& rm -rf /var/lib/apt/lists/*

# Keep the container alive; all real work happens via docker exec.
CMD ["sleep", "infinity"]
