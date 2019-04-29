#!/usr/bin/env bash

FLAG_LDFLAGS='-extldflags "-static"' xgo --targets=linux/*,darwin/*,windows-6.0/* .
