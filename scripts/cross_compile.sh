#!/usr/bin/env bash

xgo -ldflags='-extldflags "-static"' --targets=linux/*,darwin/*,windows-6.0/* .
