
# gcslock


**gcslock** is a scalable, distributed mutex that can be used to serialize
computations anywhere on the global internet. (Disclaimer: The author works
for Google but this is not an official Google product.)

## Open this repo in Google Cloud Shell

[![Open in Cloud Shell](http://gstatic.com/cloudssh/images/open-btn.png)](https://console.cloud.google.com/cloudshell/open?git_repo=https://github.com/marcacohen/gcslock&page=editor&open_in_editor=README.md)

## What is this?

Once upon a time, in CS grad school, I was given an interesting homework
assignment: using only native Unix shell commands (this was the pre-Linux era),
develop a mutual exclusion mechanism (aka, a
[mutex](https://en.wikipedia.org/wiki/Mutual_exclusion)) to serialize blocks
of commands in a shell script. Before I reveal the solution, I'd suggest 
spending some time thinking about how you might solve this problem.

The trick was to make use of the fact that file system links can be created
only once. Link creation requests have precisely the atomic "test & set"
semantics required to implement a reliable mutex. If the link doesn't already
exist, it's created, atomically, in kernel space (so no other processes can
sneak in and create the link while your request is still in progress). And
if the link already exists, the request fails. So, a reliable mutex lock can
be implemented by looping on the link creation request until it succeeds,
then doing the computation you want to protect, and finally removing the
link when you're done.

The approach described above works quite well but it also has a few problems:

1. It's inefficient because it entails relatively expensive and slow file system operations.
2. It's unreliabile: consider what happens if a script holding the lock terminates before it removes the link.

Of course, this approach was meant for a toy mutex, not something to be used
in the real world. Modern kernels offer a multitude of more efficient and more
resilient synchronization primitives. But even those mechanisms share a major
limitation with the simple file linking technique: their scope is limited to
a single operating system image.

Fast forward to more modern times. I faced a design problem that required
serializing distributed computations across multiple servers running in the
cloud. There are several well known techniques, like using Redis or a
distributed database with transaction semantics. But all of the approaches I
found seemed too complex, and required me to write too much code. Remembering
the simplicity of my homework assignment from many years ago, I wondered whether
that approach could be extended to the cloud computing era, whether atomic
file creation in the cloud could be used to build a planet-wide distributed mutex.

I dismissed Amazon's S3 as an implementation choice because it offers only
eventual consistency, meaning that object changes made at one point in time
are not guaranteed to be seen by other processes until some (typically short)
period of time in the future. Obviously this will not work for a mutex, which
requires all contending processes to see precisely the same state at the same
time. Otherwise, two processes contending for a lock could easily find themselves
both believing they had successfully acquired it.

Google Cloud Storage implements
[strong consistency](https://cloud.google.com/storage/docs/consistency)
for the relevant operations, meaning when an object is created or destroyed,
any process that reads that object, anywhere on the planet, is guaranteed
to see the change. In other words, GCS offers an integrity model that works,
at global scope, very similarly to the way a local file system works,
where we take for granted that changes to a file are guaranteed to be seen,
immediately, by all subsequent readers.

But how to implement "create once" semantics? If contending processes attempt
to create a lock file in the cloud, won't they simply overwrite each other's
objects? Well, thanks to another GCS feature,
[object versioning](https://cloud.google.com/storage/docs/object-versioning),
we can request precisely the semantics we need:

> If you set the x-goog-if-generation-match header to 0 when uploading an
> object, Cloud Storage only performs the specified request if the object
> does not currently exist. For example, you can perform a PUT request to
> create a new object with x-goog-if-generation-match set to 0, and the
> object will be created only if it doesn't already exist. Otherwise, Google
> Cloud Storage aborts the update with a status code of 412 Precondition Failed.

This is exactly the sort of guarantee provided by the kernel when we attempt
to create a file system link. The combination of strong consistency
and atomic create-once semantics gives us everything we need to build a
distributed mutex that can be used to serialize computations anywhere across
the internet.

## How do I install it?

The reference implementation in this repo is written in Go. To use gcslock
in a Go program, install the code using this command: `go get -u github.com/marcacohen/gcslock`.

The godoc document can be found [here](https://godoc.org/github.com/marcacohen/gcslock).

## How do I use it?

1. Setup a new project at the [Google APIs Console](https://console.developers.google.com) and enable the Cloud Storage API.
1. Install the [Google Cloud SDK](https://cloud.google.com/sdk/downloads) tool and configure your project and your OAuth credentials.
1. Create a bucket in which to store your lock file using the command `gsutil mb gs://your-bucket-name`.
1. Enable object versioning in your bucket using the command `gsutil versioning set on gs://your-bucket-name`.
1. In your Go code, import `github.com/marcacohen/gcslock` and use it as follows:

```go
m, err := gcslock.New(nil, bucket, object)
if err != nil {
  log.Fatal(err)
}
m.Lock()
// Protected and globally serialized computation happens here.
m.Unlock()
```

## Wait, there's more...

The Lock() and Unlock() methods implemented by this package follow the same
semantics as the standard [sync.Locker interface](https://golang.org/pkg/sync/#Locker):
they block indefinitely waiting to acquire or relinquish a lock. But sometimes
you can't afford to wait forever for something to happen. Of course, you can
implement your own timeout logic but to make life easier for clients, the mutex
object also offers the ContextLock() and ContextUnlock() methods, which accept
a context with an optional preset timeout value. If the context does not have
an associated timeout then these calls are equivalent to the Lock/Unlock methods
(i.e. they wait indefinitely to acquire/release the mutex). Here's an example 
use of locking/unlocking with a timeout context:

```go
// Instantiate mutex and setup a context with 100ms timeout.
m, err := gcslock.New(nil, bucket, object)
if err != nil {
  log.Fatal(err)
}
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

// Wait up to 100ms to acquire a lock.
if err = m.ContextLock(ctx); err != nil {
	return err
}

// Do protected work here.

// Wait up to 100ms to relinquish the lock.
if err = m.ContextUnlock(ctx); err != nil {
	return err
}
```

## Shell Script Usage

You can also use this technique via the command line using the gsutil
command, which enables serialization of shell scripts running anywhere.
The `gcslock.sh` file defines two functions, which give the ability to
globally lock shell script logic like this:
```sh
source gcslock.sh
lock mybucket
echo "protected logic"
unlock mybucket
```

## Limitations (read the fine print)

1. Performance - Because acquiring and relinquishing locks require discrete
cloud storage operations, (gross understatement coming) this is not the most
efficient mutex in the world. In practice I've found that, in the absence
of contention and retries, it requires on the order of 10 milliseconds to
acquire or relinquish a lock. This is perfectly sufficient for most batch
workloads, like my original motivating use case, but it's probably unacceptable
for any application requiring pseudo-real time responsiveness.

2. Reliability/Resilience - Unfortunately, if a process acquires the lock
and dies before relinquishing it, the entire mutex is deadlocked. In a closed
system in which you are trying to find and debug problems, this can sometimes
be the desired behavior, but for most applications this is a serious problem.
There are two ways around this issue:
    * Implement a lock watcher which periodically checks the lock age and deletes
any lock object older than a configurable threshold. Unfortunately, now you have
a new problem: what happens if the lock watcher dies?
    * Ideally the underlying storage mechanism would provide a way to automatically
delete any lock object older than a configurable interval. **Good news**: Google
Cloud Storage implements just such a feature called
[Life Cycle Management](https://cloud.google.com/storage/docs/lifecycle).
**Bad news**: The time unit, and minimum specifiable life span, is one day,
which is certaintly too long for most use cases. This experience has motivated
a feature request to the Google Cloud Storage team to support finer granularity
in specifiying object lifetimes.

## Acknowledgements

Thanks to Alex Vaghin for helping me evolve this idea and sharing his golang
expertise through many code reviews.
