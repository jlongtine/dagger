# This file generated by `mix dagger.gen`. Please DO NOT EDIT.
defmodule Dagger.CacheSharingMode do
  @moduledoc "Sharing mode of the cache volume."
  @type t() :: :LOCKED | :PRIVATE | :SHARED
  (
    @doc "Shares the cache volume amongst many build pipelines,\nbut will serialize the writes"
    @spec locked() :: :LOCKED
    def locked() do
      :LOCKED
    end
  )

  (
    @doc "Keeps a cache volume for a single build pipeline"
    @spec private() :: :PRIVATE
    def private() do
      :PRIVATE
    end
  )

  (
    @doc "Shares the cache volume amongst many build pipelines"
    @spec shared() :: :SHARED
    def shared() do
      :SHARED
    end
  )
end
