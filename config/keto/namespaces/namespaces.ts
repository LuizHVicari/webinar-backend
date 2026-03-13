import { Namespace, SubjectSet, Context } from "@ory/keto-namespace-types";

class User implements Namespace {}

class Organization implements Namespace {
  related: {
    admin: User[];
    manager: User[];
    "human-resource": User[];
    developer: User[];
  };
}
