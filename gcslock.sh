#!/bin/sh
#
# Copyright 2016 Google Inc.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#      http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This file illustrates how to the use the gcslock global mutex
# via the gsutil command in a shell script. It depends on installation
# of the Google Cloud SDK (https://cloud.google.com/sdk/), which 
# includes the gsutil command.

if [ "$BUCKET" = "" ]
then
  echo "BUCKET env var undefined"
  exit 1
fi

LOCK="gs://$BUCKET/lock"

# The lock function expects the BUCKET environment variable to be set
# to the name of a bucket writable by the user running this script.
lock() {
  sleep_time=1
  while ! echo "lock" | gsutil -q -h "x-goog-if-generation-match:0" cp - $LOCK
  do
    echo "failed to obtain lock, retrying in $sleep_time seconds"
    sleep $sleep_time
    sleep_time=$(expr $sleep_time '*' 2)
  done
}

unlock() {
  gsutil -q rm $LOCK
}

# Usage example...
lock
echo "lock acquired"
unlock
